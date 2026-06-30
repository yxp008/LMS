package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

// ============ 配置 ============
var (
	broker      = "localhost:9092"
	sourceTopic = "lms_elk_logs"
	groupID     = "lms-processor"
	chURL       = "http://localhost:8123"
	dbTable     = "LMS.LMS_Logs"
	flushSize   = 50000
	flushSecs   = 2
)

// ============ 脱敏规则 ============
type Rule struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Desc        string `json:"desc"`
}
type RuleConfig struct{ Rules []Rule }

var compiledRules []struct {
	re   *regexp.Regexp
	repl string
}

func loadRules(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg RuleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	for _, r := range cfg.Rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			log.Printf("[PROCESSOR] 规则编译失败: %s", r.Desc)
			continue
		}
		compiledRules = append(compiledRules, struct {
			re   *regexp.Regexp
			repl string
		}{re, r.Replacement})
		log.Printf("[PROCESSOR] 已加载规则: %s", r.Desc)
	}
	return nil
}

// ============ 脱敏 ============
func desensitize(raw map[string]interface{}) {
	for _, field := range []string{"message", "syslog_message"} {
		if v, ok := raw[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				for _, r := range compiledRules {
					s = r.re.ReplaceAllString(s, r.repl)
				}
				raw[field] = s
			}
		}
	}
}

// ============ 字段解析 (对应 VRL process_elk) ============
func generateLogID() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	randS := fmt.Sprintf("%04d", 1000+n.Int64())
	ts := time.Now().UnixMilli()
	return fmt.Sprintf("L%d%s", ts, randS)
}

func parseTimestamp(src map[string]interface{}) time.Time {
	raw, ok := src["@timestamp"]
	if !ok {
		raw, ok = src["timestamp"]
	}
	if ok {
		if s, ok := raw.(string); ok {
			layouts := []string{
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05.000Z",
				"2006-01-02T15:04:05-07:00",
				time.RFC3339,
			}
			for _, l := range layouts {
				if t, err := time.Parse(l, s); err == nil {
					return t
				}
			}
		}
	}
	return time.Now()
}

func parseLevel(src map[string]interface{}) string {
	raw := ""
	for _, k := range []string{"level", "severity", "log_level"} {
		if v, ok := src[k]; ok {
			raw = strings.ToLower(fmt.Sprint(v))
			break
		}
	}
	switch raw {
	case "error", "err", "critical", "fatal", "3":
		return "3"
	case "warn", "warning", "2":
		return "2"
	case "debug", "4":
		return "4"
	default:
		return "1"
	}
}

func parseHost(src map[string]interface{}) string {
	if h, ok := src["host"]; ok {
		switch v := h.(type) {
		case map[string]interface{}:
			if ip, ok := v["ip"]; ok {
				return fmt.Sprint(ip)
			}
			if hn, ok := v["hostname"]; ok {
				return fmt.Sprint(hn)
			}
		case string:
			return v
		}
	}
	if hn, ok := src["syslog_hostname"]; ok {
		return fmt.Sprint(hn)
	}
	if hn, ok := src["hostname"]; ok {
		return fmt.Sprint(hn)
	}
	if hn, ok := src["host_name"]; ok {
		return fmt.Sprint(hn)
	}
	return "unknown"
}

func parseMessage(src map[string]interface{}) string {
	for _, k := range []string{"syslog_message", "message", "msg", "log"} {
		if v, ok := src[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// ============ ClickHouse 批量写入 ============
func insertBatch(rows []map[string]interface{}) error {
	var sb strings.Builder
	for _, r := range rows {
		b, _ := json.Marshal(r)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	url := fmt.Sprintf("%s/?query=INSERT%%20INTO%%20%s%%20FORMAT%%20JSONEachRow", chURL, dbTable)
	req, err := http.NewRequest("POST", url, strings.NewReader(sb.String()))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("CH error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ============ 主程序 ============
func main() {
	exeDir, _ := os.Executable()
	rulesPath := filepath.Join(filepath.Dir(exeDir), "rules.json")
	if len(os.Args) > 1 {
		rulesPath = os.Args[1]
	}
	if err := loadRules(rulesPath); err != nil {
		log.Fatalf("[PROCESSOR] 加载规则失败: %v", err)
	}

	log.Printf("[PROCESSOR] 启动: %s/%s → ClickHouse %s", broker, sourceTopic, dbTable)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{broker},
		Topic:       sourceTopic,
		GroupID:     groupID,
		StartOffset: kafka.LastOffset,
		MaxWait:     1 * time.Second,
		MaxBytes:    10e6,
	})
	defer reader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[PROCESSOR] 收到停止信号...")
		cancel()
	}()

	batch := make([]map[string]interface{}, 0, flushSize)
	ticker := time.NewTicker(time.Duration(flushSecs) * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := insertBatch(batch); err != nil {
			log.Printf("[PROCESSOR] 写入失败: %v", err)
			return
		}
		log.Printf("[PROCESSOR] 写入 %d 条", len(batch))
		batch = batch[:0]
	}

	count := int64(0)
	for {
		select {
		case <-ctx.Done():
			flush()
			log.Printf("[PROCESSOR] 已停止，共处理 %d 条", count)
			return
		case <-ticker.C:
			flush()
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "deadline") {
				continue
			}
			log.Printf("[PROCESSOR] 读取失败: %v", err)
			time.Sleep(time.Second)
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(msg.Value, &raw); err != nil {
			continue
		}

		// 处理 ELK _source 包装
		src := raw
		if v, ok := raw["_source"]; ok {
			if m, ok := v.(map[string]interface{}); ok {
				src = m
			}
		}

		// 脱敏
		desensitize(src)

		// 提取标准字段
		ts := time.Now()
		host := parseHost(src)
		msgText := parseMessage(src)
		level := parseLevel(src)

		// Tags: 移除已映射字段后的剩余
		delete(src, "@timestamp")
		delete(src, "timestamp")
		delete(src, "level")
		delete(src, "severity")
		delete(src, "log_level")
		delete(src, "host")
		delete(src, "hostname")
		delete(src, "host_name")
		delete(src, "syslog_hostname")
		delete(src, "message")
		delete(src, "syslog_message")
		delete(src, "msg")
		delete(src, "log")
		tagsJSON, _ := json.Marshal(src)
		if string(tagsJSON) == "null" || string(tagsJSON) == "{}" {
			tagsJSON = []byte("{}")
		}

		batch = append(batch, map[string]interface{}{
			"Log_ID":      generateLogID(),
			"Timestamp":   ts.Format("2006-01-02 15:04:05"),
			"Level":       level,
			"Host":        host,
			"Source_Type": "ELK本地日志文件",
			"Message":     msgText,
			"Tags":        json.RawMessage(tagsJSON),
			"Collector_ID": "C001",
		})

		count++
		if len(batch) >= flushSize {
			flush()
		}
	}
}
