package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ============ 配置常量 ============
var (
	projectRoot   = getProjectRoot()
	frontendDir   = filepath.Join(projectRoot, "frontend")
	chURL         = "http://localhost:8123"
	database      = "LMS"
	table         = "LMS_Logs"
	prefsFile     = filepath.Join(projectRoot, "collector", "collection_prefs.json")
	vectorTpl     = filepath.Join(projectRoot, "collector", "vector_wsl.toml.template")
	vectorCfg     = filepath.Join(projectRoot, "collector", "vector_wsl.toml")
	vectorBin     = filepath.Join(os.Getenv("HOME"), ".vector", "bin", "vector")
	vectorPID     = "/tmp/vector.pid"
	vectorLog     = "/tmp/vector.log"
	smtpCfgFile   = filepath.Join(frontendDir, "smtp_config.json")
	listenAddr    = ":8080"
	collectorStateFile = filepath.Join(projectRoot, "collector", "collector_state.json")

)

func getProjectRoot() string {
	// 环境变量优先
	if root := os.Getenv("LMS_PROJECT_ROOT"); root != "" {
		return root
	}
	// 编译后二进制在 frontend/ 下，父目录即项目根
	exe, _ := os.Executable()
	d := filepath.Dir(filepath.Dir(exe))
	if _, err := os.Stat(filepath.Join(d, "frontend", "server.go")); err == nil {
		return d
	}
	// 直接找二进制所在目录的父目录
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "..", "frontend", "server.go")); err == nil {
		return filepath.Clean(filepath.Join(filepath.Dir(exe), ".."))
	}
	// 最后回退到当前工作目录
	cwd, _ := os.Getwd()
	return cwd
}

// ============ 采集偏好 ============
type CollectionPrefs struct {
	LinuxSystemLogs    bool   `json:"linux_system_logs"`
	NetworkDeviceLogs  bool   `json:"network_device_logs"`
	ElkFileLogs        bool   `json:"elk_file_logs"`
	ElkFilePath        string `json:"elk_file_path"`
}

func loadPrefs() CollectionPrefs {
	prefs := CollectionPrefs{
		LinuxSystemLogs: false, NetworkDeviceLogs: false,
		ElkFileLogs: true,
		ElkFilePath: filepath.Join(projectRoot, "collector/elk_logs/incoming/"),
	}
	data, err := os.ReadFile(prefsFile)
	if err != nil {
		return prefs
	}
	var loaded CollectionPrefs
	if json.Unmarshal(data, &loaded) == nil {
		if loaded.ElkFilePath != "" { prefs.ElkFilePath = loaded.ElkFilePath }
		prefs.LinuxSystemLogs = loaded.LinuxSystemLogs
		prefs.NetworkDeviceLogs = loaded.NetworkDeviceLogs
		prefs.ElkFileLogs = loaded.ElkFileLogs
	}
	return prefs
}

func savePrefs(prefs CollectionPrefs) {
	os.MkdirAll(filepath.Dir(prefsFile), 0755)
	data, _ := json.MarshalIndent(prefs, "", "  ")
	os.WriteFile(prefsFile, data, 0644)
}

// ============ Vector 配置生成 ============
func generateVectorConfig(prefs CollectionPrefs) {
	text, err := os.ReadFile(vectorTpl)
	if err != nil {
		log.Printf("[SERVER] 读取模板失败: %v", err)
		return
	}
	lines := strings.Split(string(text), "\n")
	var output []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if m := regexp.MustCompile(`^# ===== COLLECTION:\s*(\w+)\s*=====$`).FindStringSubmatch(line); m != nil {
			collType := m[1]
			i++
			var block []string
			for i < len(lines) && !regexp.MustCompile(`^# ===== END COLLECTION\s*=====$`).MatchString(lines[i]) {
				block = append(block, lines[i])
				i++
			}
			i++ // skip END
			switch collType {
			case "linux_system_logs":
				if prefs.LinuxSystemLogs { output = append(output, block...) }
			case "network_device_logs":
				if prefs.NetworkDeviceLogs { output = append(output, block...) }
			case "elk_file_logs":
				if prefs.ElkFileLogs { output = append(output, block...) }
			}
			continue
		}
		if regexp.MustCompile(`^# ===== INPUTS\s*=====$`).MatchString(line) {
			i++
			var inputsList []string
			if prefs.LinuxSystemLogs { inputsList = append(inputsList, `"cleanup_journald"`) }
			if prefs.NetworkDeviceLogs { inputsList = append(inputsList, `"cleanup_syslog"`) }
			var block []string
			for i < len(lines) && !regexp.MustCompile(`^# ===== END INPUTS\s*=====$`).MatchString(lines[i]) {
				if regexp.MustCompile(`^\s*inputs\s*=`).MatchString(lines[i]) {
					if len(inputsList) > 0 {
						block = append(block, fmt.Sprintf("inputs = [%s]", strings.Join(inputsList, ", ")))
					}
				} else if len(inputsList) > 0 {
					block = append(block, lines[i])
				}
				i++
			}
			i++ // skip END
			output = append(output, block...)
			continue
		}
		output = append(output, line)
		i++
	}
	cfg := strings.Join(output, "\n") + "\n"
	cfg = strings.ReplaceAll(cfg, "__PROJECT_ROOT__", projectRoot)
	cfg = strings.ReplaceAll(cfg, "__ELK_FILE_PATH__", prefs.ElkFilePath)
	os.MkdirAll(filepath.Dir(vectorCfg), 0755)
	os.WriteFile(vectorCfg, []byte(cfg), 0644)
}

// ============ Vector 进程管理 ============
func vectorIsRunning() bool {
	pidData, err := os.ReadFile(vectorPID)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return false
	}
	statusData, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return false
	}
	return strings.Contains(string(statusData), "State:") && !strings.Contains(string(statusData), "State:\tZ")
}

func killAllVector() {
	exec.Command("pkill", "-9", "-x", "vector").Run()
	time.Sleep(2 * time.Second)
}

func stopVector() {
	killAllVector()
	os.Remove(vectorPID)
	log.Println("[SERVER] Vector 已停止")
}

func startVector() {
	f, _ := os.OpenFile(vectorLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	logFile, _ := os.OpenFile(vectorLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	cmd := exec.Command(vectorBin, "--config", vectorCfg)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Start()
	go func() {
		cmd.Wait()
		logFile.Close()
	}()
	time.Sleep(3 * time.Second)
	out, _ := exec.Command("pgrep", "-x", "vector").Output()
	realPID := strings.TrimSpace(string(out))
	if realPID == "" {
		realPID = fmt.Sprintf("%d", cmd.Process.Pid)
	}
	os.WriteFile(vectorPID, []byte(realPID), 0644)
	cq(`ALTER TABLE `+database+`.LMS_Collectors UPDATE Status = '1' WHERE 1=1`)
	log.Printf("[SERVER] Vector 已启用, PID=%s", realPID)
}

func restartVector(prefs CollectionPrefs) {
	anyEnabled := prefs.LinuxSystemLogs || prefs.NetworkDeviceLogs || prefs.ElkFileLogs
	if anyEnabled {
		generateVectorConfig(prefs)
		stopVector()
		startVector()
	} else {
		stopVector()
		cq(`ALTER TABLE ` + database + `.LMS_Collectors UPDATE Status = '0' WHERE 1=1`)
		log.Println("[SERVER] 所有采集类型已禁用，Vector 不启动")
	}
}

// ============ ClickHouse 查询 ============
func cq(sql string) []map[string]interface{} {
	resp, err := http.Post(chURL+"/?default_format=JSONEachRow", "text/plain", strings.NewReader(sql))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var results []map[string]interface{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" { continue }
		var m map[string]interface{}
		if json.Unmarshal([]byte(line), &m) == nil {
			results = append(results, m)
		}
	}
	return results
}

// ============ HTTP 处理器 ============
func jsonResp(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func apiStats(w http.ResponseWriter, r *http.Request) {
	sql := fmt.Sprintf(`SELECT count() as total_logs, countIf(Level='1') as info_count, countIf(Level='2') as warn_count, countIf(Level='3') as error_count, countIf(Level='4') as debug_count, countIf(Timestamp>=now()-INTERVAL 1 DAY) as last_24h, countIf(Timestamp>=now()-INTERVAL 1 HOUR) as last_1h, min(Timestamp) as earliest, max(Timestamp) as latest, uniq(Host) as host_count, uniq(Source_Type) as source_count FROM %s.%s`, database, table)
	results := cq(sql)
	if len(results) > 0 {
		jsonResp(w, 200, results[0])
	} else {
		jsonResp(w, 200, map[string]interface{}{"total_logs": 0, "info_count": 0, "warn_count": 0, "error_count": 0, "debug_count": 0, "last_24h": 0, "last_1h": 0, "earliest": "", "latest": "", "host_count": 0, "source_count": 0})
	}
}

func apiLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page")); if page < 1 { page = 1 }
	pageSize, _ := strconv.Atoi(q.Get("page_size")); if pageSize < 1 { pageSize = 50 }
	level := q.Get("level")
	host := q.Get("host")
	sourceType := q.Get("source_type")
	search := q.Get("search")
	startTime := q.Get("start_time")
	endTime := q.Get("end_time")
	offset := (page - 1) * pageSize

	var conditions []string
	if level != "" { conditions = append(conditions, fmt.Sprintf("Level = '%s'", strings.ReplaceAll(level, "'", "\\'"))) }
	if host != "" { conditions = append(conditions, fmt.Sprintf("Host = '%s'", strings.ReplaceAll(host, "'", "\\'"))) }
	if sourceType != "" { conditions = append(conditions, fmt.Sprintf("Source_Type = '%s'", strings.ReplaceAll(sourceType, "'", "\\'"))) }
	if search != "" { conditions = append(conditions, fmt.Sprintf("Message LIKE '%%%s%%'", strings.ReplaceAll(search, "'", "\\'"))) }
	if startTime != "" { conditions = append(conditions, fmt.Sprintf("Timestamp >= '%s'", strings.ReplaceAll(startTime, "'", "\\'"))) }
	if endTime != "" { conditions = append(conditions, fmt.Sprintf("Timestamp <= '%s'", strings.ReplaceAll(endTime, "'", "\\'"))) }
	where := ""
	if len(conditions) > 0 { where = "WHERE " + strings.Join(conditions, " AND ") }

	totalResults := cq(fmt.Sprintf("SELECT count() as total FROM %s.%s %s", database, table, where))
	total := 0
	if len(totalResults) > 0 {
		if v, ok := totalResults[0]["total"].(float64); ok { total = int(v) }
	}
	logs := cq(fmt.Sprintf("SELECT Log_ID, Timestamp, Level, Host, Source_Type, Message, Tags, Collector_ID FROM %s.%s %s ORDER BY Timestamp DESC LIMIT %d OFFSET %d", database, table, where, pageSize, offset))
	totalPages := 0
	if total > 0 { totalPages = (total + pageSize - 1) / pageSize }

	jsonResp(w, 200, map[string]interface{}{
		"total": total, "page": page, "page_size": pageSize, "total_pages": totalPages, "data": logs,
	})
}

func apiLevels(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, 200, cq(fmt.Sprintf("SELECT Level, count() as count FROM %s.%s GROUP BY Level ORDER BY Level", database, table)))
}

func apiTimeline(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours")); if hours < 1 { hours = 24 }
	interval := hours / 24; if interval < 1 { interval = 1 }
	jsonResp(w, 200, cq(fmt.Sprintf("SELECT toStartOfInterval(Timestamp, INTERVAL %d HOUR) as time_point, Level, count() as count FROM %s.%s WHERE Timestamp >= now() - INTERVAL %d HOUR GROUP BY time_point, Level ORDER BY time_point", interval, database, table, hours)))
}

func apiSources(w http.ResponseWriter, r *http.Request) {
	r2 := cq(fmt.Sprintf("SELECT Source_Type, count() as count FROM %s.%s GROUP BY Source_Type ORDER BY count DESC", database, table))
	existing := map[string]bool{}
	for _, item := range r2 {
		if s, ok := item["Source_Type"].(string); ok { existing[s] = true }
	}
	for _, t := range []string{"Linux系统日志", "网络设备日志", "ELK本地日志文件"} {
		if !existing[t] { r2 = append(r2, map[string]interface{}{"Source_Type": t, "count": float64(0)}) }
	}
	jsonResp(w, 200, r2)
}

func apiHosts(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("detail") == "1" {
		jsonResp(w, 200, cq(fmt.Sprintf("SELECT Host, Level, count() as count FROM %s.%s GROUP BY Host, Level ORDER BY count DESC", database, table)))
	} else {
		jsonResp(w, 200, cq(fmt.Sprintf("SELECT Host, count() as count FROM %s.%s GROUP BY Host ORDER BY count DESC LIMIT 20", database, table)))
	}
}

func apiCollectors(w http.ResponseWriter, r *http.Request) {
	results := cq(fmt.Sprintf("SELECT * FROM %s.LMS_Collectors ORDER BY Collector_ID", database))
	prefs := loadPrefs()
	anyEnabled := prefs.LinuxSystemLogs || prefs.NetworkDeviceLogs || prefs.ElkFileLogs
	actualStatus := "0"
	if anyEnabled && vectorIsRunning() { actualStatus = "1" }
	for _, r := range results {
		r["Status"] = actualStatus
		r["Source_Types"] = []map[string]interface{}{
			{"name": "Linux系统日志", "key": "linux_system_logs", "enabled": prefs.LinuxSystemLogs},
			{"name": "网络设备日志", "key": "network_device_logs", "enabled": prefs.NetworkDeviceLogs},
			{"name": "ELK本地日志文件", "key": "elk_file_logs", "enabled": prefs.ElkFileLogs},
		}
	}
	jsonResp(w, 200, results)
}

func apiCollectorsPost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	action, _ := data["action"].(string)
	if action == "create" {
		cid, _ := data["Collector_ID"].(string)
		name, _ := data["Name"].(string)
		addr, _ := data["Address"].(string)
		if cid == "" || name == "" { jsonResp(w, 400, map[string]string{"error": "missing fields"}); return }
		cq(fmt.Sprintf("INSERT INTO %s.LMS_Collectors VALUES ('%s','%s','1','%s')", database, cid, name, addr))
		jsonResp(w, 200, map[string]interface{}{"ok": true, "Collector_ID": cid})
	} else if action == "delete" {
		cid, _ := data["Collector_ID"].(string)
		if cid == "" { jsonResp(w, 400, map[string]string{"error": "missing Collector_ID"}); return }
		cq(fmt.Sprintf("ALTER TABLE %s.LMS_Collectors UPDATE Status = '0' WHERE Collector_ID = '%s'", database, cid))
		cq(fmt.Sprintf("DELETE FROM %s.LMS_Collectors WHERE Collector_ID = '%s'", database, cid))
		log.Printf("[SERVER] 采集器 %s 已断开并删除", cid)
		jsonResp(w, 200, map[string]bool{"ok": true})
	} else if action == "update_status" {
		cid, _ := data["Collector_ID"].(string)
		status, _ := data["Status"].(string)
		if cid == "" { jsonResp(w, 400, map[string]string{"error": "missing Collector_ID"}); return }
		cq(fmt.Sprintf("ALTER TABLE %s.LMS_Collectors UPDATE Status = '%s' WHERE Collector_ID = '%s'", database, status, cid))
		if status == "0" { stopVector(); log.Printf("[SERVER] 采集器 %s 已停用", cid) } else {
			prefs := loadPrefs(); generateVectorConfig(prefs); startVector(); log.Printf("[SERVER] 采集器 %s 已启用", cid)
		}
		jsonResp(w, 200, map[string]bool{"ok": true})
	} else { jsonResp(w, 400, map[string]string{"error": "unknown action"}) }
}

func apiCollectionPrefsGet(w http.ResponseWriter, r *http.Request) { jsonResp(w, 200, loadPrefs()) }

func apiCollectionPrefsPost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	prefs := loadPrefs()
	if v, ok := data["linux_system_logs"].(bool); ok { prefs.LinuxSystemLogs = v }
	if v, ok := data["network_device_logs"].(bool); ok { prefs.NetworkDeviceLogs = v }
	if v, ok := data["elk_file_logs"].(bool); ok { prefs.ElkFileLogs = v }
	if v, ok := data["elk_file_path"].(string); ok && v != "" { prefs.ElkFilePath = v }
	savePrefs(prefs)
	restartVector(prefs)
	any := prefs.LinuxSystemLogs || prefs.NetworkDeviceLogs || prefs.ElkFileLogs
	jsonResp(w, 200, map[string]interface{}{"ok": true, "vector_running": any})
}

func apiAlertRules(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, 200, cq(fmt.Sprintf("SELECT * FROM %s.LMS_AlertRules ORDER BY AlertRule_ID", database)))
}

func apiAlertRulesPost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	action, _ := data["action"].(string)
	esc := func(s string) string { return strings.ReplaceAll(s, "'", "\\'") }
	switch action {
	case "create":
		name := esc(fmt.Sprint(data["Name"]))
		desc := esc(fmt.Sprint(data["Desc"]))
		sql := esc(fmt.Sprint(data["Alert_Sql"]))
		interval := esc(fmt.Sprint(data["Interval"]))
		channel, _ := data["Channel"].(string); if channel == "" { channel = "1" }
		address := esc(fmt.Sprint(data["Address"]))
		level, _ := data["Level"].(string); if level == "" { level = "3" }
		status, _ := data["Status"].(string); if status == "" { status = "1" }
		existing := cq(fmt.Sprintf("SELECT max(AlertRule_ID) as max_id FROM %s.LMS_AlertRules", database))
		maxNum := 1
		if len(existing) > 0 { if mid, ok := existing[0]["max_id"].(string); ok {
			id := strings.TrimPrefix(mid, "AR"); if n, e := strconv.Atoi(id); e == nil { maxNum = n + 1 }
		}}
		ruleID := fmt.Sprintf("AR%03d", maxNum)
		cq(fmt.Sprintf("INSERT INTO %s.LMS_AlertRules VALUES ('%s','%s','%s','%s','%s','%s','%s',now(),now(),'%s','%s')", database, ruleID, name, desc, sql, interval, channel, address, level, status))
		jsonResp(w, 200, map[string]interface{}{"ok": true, "AlertRule_ID": ruleID})
	case "update":
		ruleID := esc(fmt.Sprint(data["AlertRule_ID"]))
		var fields []string
		for _, k := range []string{"Name", "Desc", "Alert_Sql", "Interval", "Channel", "Address", "Level", "Status"} {
			if v, ok := data[k]; ok { fields = append(fields, fmt.Sprintf("%s = '%s'", k, esc(fmt.Sprint(v)))) }
		}
		fields = append(fields, "Updated_Time = now()")
		cq(fmt.Sprintf("ALTER TABLE %s.LMS_AlertRules UPDATE %s WHERE AlertRule_ID = '%s'", database, strings.Join(fields, ", "), ruleID))
		jsonResp(w, 200, map[string]bool{"ok": true})
	case "delete":
		ruleID := esc(fmt.Sprint(data["AlertRule_ID"]))
		cq(fmt.Sprintf("DELETE FROM %s.LMS_AlertRules WHERE AlertRule_ID = '%s'", database, ruleID))
		jsonResp(w, 200, map[string]bool{"ok": true})
	default:
		jsonResp(w, 400, map[string]string{"error": "unknown action"})
	}
}

func apiSMTPConfigGet(w http.ResponseWriter, r *http.Request) {
	if data, err := os.ReadFile(smtpCfgFile); err == nil {
		var cfg map[string]interface{}
		if json.Unmarshal(data, &cfg) == nil { cfg["password"] = ""; jsonResp(w, 200, cfg); return }
	}
	jsonResp(w, 200, map[string]interface{}{"host": "", "port": 465, "sender": "", "password": ""})
}

func apiSMTPConfigPost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	port, _ := data["port"].(float64)
	if port == 0 { port = 465 }
	cfg := map[string]interface{}{"host": data["host"], "port": port, "sender": data["sender"], "password": data["password"], "use_ssl": true}
	bytes, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(smtpCfgFile, bytes, 0644)
	jsonResp(w, 200, map[string]bool{"ok": true})
}

func apiSMTPTest(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	host, _ := data["host"].(string)
	port, _ := data["port"].(float64)
	sender, _ := data["sender"].(string)
	password, _ := data["password"].(string)
	if host == "" || sender == "" || password == "" { jsonResp(w, 400, map[string]string{"error": "请填写所有必填字段"}); return }
	addr := fmt.Sprintf("%s:%d", host, int(port))
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil { jsonResp(w, 200, map[string]interface{}{"ok": false, "error": fmt.Sprintf("连接失败: %v", err)}); return }
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil { jsonResp(w, 200, map[string]interface{}{"ok": false, "error": fmt.Sprintf("SMTP连接失败: %v", err)}); return }
	auth := smtp.PlainAuth("", sender, password, host)
	if err := client.Auth(auth); err != nil { jsonResp(w, 200, map[string]interface{}{"ok": false, "error": "认证失败: 请检查邮箱和授权码"}); return }
	jsonResp(w, 200, map[string]bool{"ok": true})
}

func apiSQLValidatePost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	sql, _ := data["sql"].(string)
	if sql == "" { jsonResp(w, 200, map[string]interface{}{"ok": false, "error": "SQL empty"}); return }
	testSQL := fmt.Sprintf("SELECT * FROM (%s) LIMIT 1 FORMAT JSONEachRow", sql)
	resp, err := http.Post(chURL+"/?default_format=JSONEachRow", "text/plain", strings.NewReader(testSQL))
	if err != nil { jsonResp(w, 200, map[string]interface{}{"ok": false, "error": err.Error()}); return }
	defer resp.Body.Close()
	if resp.StatusCode == 200 { jsonResp(w, 200, map[string]interface{}{"ok": true, "message": "SQL OK"}) } else {
		body, _ := io.ReadAll(resp.Body)
		jsonResp(w, 200, map[string]interface{}{"ok": false, "error": string(body)})
	}
}

func apiWebhook(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	alertName := fmt.Sprint(data["alert"]); if alertName == "" { alertName = "未知告警" }
	count, _ := data["count"].(float64)
	level, _ := data["level"].(string)
	ruleID, _ := data["rule_id"].(string)
	log.Printf("[SERVER] Webhook 收到告警: %s, 匹配 %.0f 条, 等级 %s", alertName, count, level)
	triggerID := fmt.Sprintf("T%d", time.Now().UnixMilli())
	msg := fmt.Sprintf("Webhook告警触发: %s 匹配%.0f条", alertName, count)
	cq(fmt.Sprintf("INSERT INTO %s.LMS_AlertTriggers VALUES ('%s','%s','%s',now(),%.0f,'3','webhook','%s')", database, triggerID, ruleID, alertName, count, msg))
	jsonResp(w, 200, map[string]interface{}{"ok": true, "message": "Webhook received"})
}

func apiAlertTriggers(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, 200, cq(fmt.Sprintf(`SELECT r.AlertRule_ID, r.Name as Rule_Name, r.Desc, r.Alert_Sql, r.Interval, r.Channel, r.Address, r.Level, r.Status, count(t.Trigger_ID) as Trigger_Count, max(t.Trigger_Time) as Latest_Time, max(t.Match_Count) as Last_Match_Count FROM %s.LMS_AlertRules r LEFT JOIN %s.LMS_AlertTriggers t ON r.AlertRule_ID = t.AlertRule_ID GROUP BY r.AlertRule_ID, r.Name, r.Desc, r.Alert_Sql, r.Interval, r.Channel, r.Address, r.Level, r.Status ORDER BY Trigger_Count DESC, r.AlertRule_ID`, database, database)))
}

// ============ 静态文件 ============
func staticHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || path == "" { path = "/index.html" }
	http.ServeFile(w, r, filepath.Join(frontendDir, path))
}

// ============ 路由 ============
// ============ 客户端本地存储 API（不依赖 ClickHouse） ============
func apiCollectorsLocal(w http.ResponseWriter, r *http.Request) {
	cs := loadCollectorState()
	prefs := loadPrefs()
	anyEnabled := prefs.LinuxSystemLogs || prefs.NetworkDeviceLogs || prefs.ElkFileLogs
	actualStatus := "0"
	if anyEnabled && vectorIsRunning() { actualStatus = "1" }
	result := map[string]interface{}{
		"Collector_ID": cs.CollectorID, "Name": cs.Name, "Status": actualStatus, "Address": cs.Address,
		"Source_Types": []map[string]interface{}{
			{"name": "Linux系统日志", "key": "linux_system_logs", "enabled": prefs.LinuxSystemLogs},
			{"name": "网络设备日志", "key": "network_device_logs", "enabled": prefs.NetworkDeviceLogs},
			{"name": "ELK本地日志文件", "key": "elk_file_logs", "enabled": prefs.ElkFileLogs},
		},
	}
	jsonResp(w, 200, []map[string]interface{}{result})
}

func apiCollectorsLocalPost(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	action, _ := data["action"].(string)
	if action == "update_status" {
		status, _ := data["Status"].(string)
		if status == "0" { stopVector() } else {
			prefs := loadPrefs(); generateVectorConfig(prefs); startVector()
		}
		jsonResp(w, 200, map[string]bool{"ok": true})
	} else {
		jsonResp(w, 400, map[string]string{"error": "unknown action"})
	}
}

// ============ 采集器路由（8081） ============
func collectorRouter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" { w.WriteHeader(200); return }

	path := strings.Split(r.URL.Path, "?")[0]
	if r.Method == "GET" {
		switch path {
		case "/api/collection-prefs": apiCollectionPrefsGet(w, r)
		case "/api/collectors": apiCollectorsLocal(w, r)
		default: staticHandler(w, r)
		}
		return
	}
	if r.Method == "POST" {
		switch path {
		case "/api/collection-prefs": apiCollectionPrefsPost(w, r)
		case "/api/collectors": apiCollectorsLocalPost(w, r)
		default: http.NotFound(w, r)
		}
		return
	}
	staticHandler(w, r)
}

func apiCollectorsReadonly(w http.ResponseWriter, r *http.Request) {
	results := cq(fmt.Sprintf("SELECT * FROM %s.LMS_Collectors ORDER BY Collector_ID", database))
	prefs := loadPrefs()
	for _, r := range results {
		r["Source_Types"] = []map[string]interface{}{
			{"name": "Linux系统日志", "key": "linux_system_logs", "enabled": prefs.LinuxSystemLogs},
			{"name": "网络设备日志", "key": "network_device_logs", "enabled": prefs.NetworkDeviceLogs},
			{"name": "ELK本地日志文件", "key": "elk_file_logs", "enabled": prefs.ElkFileLogs},
		}
	}
	jsonResp(w, 200, results)
}

// ============ 服务端路由（8080） ============
func serverRouter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" { w.WriteHeader(200); return }

	path := strings.Split(r.URL.Path, "?")[0]
	if !strings.HasPrefix(path, "/api/") { staticHandler(w, r); return }

	if r.Method == "GET" {
		switch path {
		case "/api/stats": apiStats(w, r)
		case "/api/logs": apiLogs(w, r)
		case "/api/levels": apiLevels(w, r)
		case "/api/timeline": apiTimeline(w, r)
		case "/api/sources": apiSources(w, r)
		case "/api/hosts": apiHosts(w, r)
		case "/api/collectors": apiCollectors(w, r)
		case "/api/collection-prefs": apiCollectionPrefsGet(w, r)
		case "/api/alert-rules": apiAlertRules(w, r)
		case "/api/smtp-config": apiSMTPConfigGet(w, r)
		case "/api/alert-triggers": apiAlertTriggers(w, r)
		default: http.NotFound(w, r)
		}
		return
	}
	if r.Method == "POST" {
		switch path {
		case "/api/collectors": apiCollectorsPost(w, r)
		case "/api/collection-prefs": apiCollectionPrefsPost(w, r)
		case "/api/alert-rules": apiAlertRulesPost(w, r)
		case "/api/smtp-config": apiSMTPConfigPost(w, r)
		case "/api/smtp-test": apiSMTPTest(w, r)
		case "/api/sql-validate": apiSQLValidatePost(w, r)
		case "/api/webhook": apiWebhook(w, r)
		case "/api/query":
			var data map[string]interface{}
			json.NewDecoder(r.Body).Decode(&data)
			sql, _ := data["sql"].(string)
			jsonResp(w, 200, cq(sql))
		default: http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}

// ============ 告警检查器 (goroutine) ============
func alertChecker() {
	log.Println("[ALERT] 告警检查器已启动")
	for {
		time.Sleep(5 * time.Second)
		rules := cq(fmt.Sprintf("SELECT * FROM %s.LMS_AlertRules WHERE Status = '1'", database))
		for _, rule := range rules {
			ruleID, _ := rule["AlertRule_ID"].(string)
			name, _ := rule["Name"].(string)
			alertSQL, _ := rule["Alert_Sql"].(string)
			channel, _ := rule["Channel"].(string)
			address, _ := rule["Address"].(string)
			if alertSQL == "" || address == "" { continue }

			results := cq(alertSQL)
			if len(results) == 0 { continue }
			count := 0
			for _, v := range results[0] {
				if n, ok := v.(float64); ok { count = int(n); break }
			}
			if count == 0 { continue }

			levelMap := map[string]string{"1": "严重", "2": "高", "3": "中", "4": "低"}
			level, _ := rule["Level"].(string)
			triggerID := fmt.Sprintf("T%d", time.Now().UnixMilli())
			safeName := strings.ReplaceAll(name, "'", "\\'")
			msg := fmt.Sprintf("告警触发: %s 匹配%d条记录", safeName, count)
			cq(fmt.Sprintf("INSERT INTO %s.LMS_AlertTriggers VALUES ('%s','%s','%s',now(),%d,'%s','%s','%s')", database, triggerID, ruleID, safeName, count, channel, address, msg))
			log.Printf("[ALERT] 触发告警: %s (匹配 %d 条)", name, count)

			if channel == "1" { // 邮件
				var smtpCfg map[string]interface{}
				if data, err := os.ReadFile(smtpCfgFile); err == nil {
					json.Unmarshal(data, &smtpCfg)
					host, _ := smtpCfg["host"].(string)
					port, _ := smtpCfg["port"].(float64)
					sender, _ := smtpCfg["sender"].(string)
					password, _ := smtpCfg["password"].(string)
					if host != "" && sender != "" && password != "" {
						addr := fmt.Sprintf("%s:%d", host, int(port))
						conn, _ := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
						if conn != nil {
							client, _ := smtp.NewClient(conn, host)
							if client != nil {
								auth := smtp.PlainAuth("", sender, password, host)
								client.Auth(auth)
								body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: [LMS告警] %s - %s\r\n\r\n%s", sender, address, name, levelMap[level], msg)
								client.Mail(sender)
								client.Rcpt(address)
								wc, _ := client.Data()
								if wc != nil { wc.Write([]byte(body)); wc.Close() }
								client.Quit()
							}
							conn.Close()
						}
					}
				}
			} else if channel == "3" { // Webhook
				body, _ := json.Marshal(map[string]interface{}{"alert": name, "count": count, "level": levelMap[level], "rule_id": ruleID})
				http.Post(address, "application/json", strings.NewReader(string(body)))
			}
		}
	}
}

// ============ 主程序 ============
func main() {
	collectorMode := flag.Bool("collector", false, "采集器管理模式")
	flag.Parse()

	if *collectorMode {
		listenAddr = ":8081"
		// 确保本地状态文件存在
		loadCollectorState()
		saveCollectorState(loadCollectorState())
		prefs := loadPrefs()
		anyEnabled := prefs.LinuxSystemLogs || prefs.NetworkDeviceLogs || prefs.ElkFileLogs
		if anyEnabled {
			generateVectorConfig(prefs)
			startVector()
		}
		log.Printf("[COLLECTOR] 采集器管理: http://localhost%s", listenAddr)
		log.Fatal(http.ListenAndServe(listenAddr, http.HandlerFunc(collectorRouter)))
	}

	prefs := loadPrefs()
	anyEnabled := prefs.LinuxSystemLogs || prefs.NetworkDeviceLogs || prefs.ElkFileLogs
	if anyEnabled {
		generateVectorConfig(prefs)
		startVector()
	} else { log.Println("[SERVER] 所有采集类型已禁用，Vector 不启动") }

	go alertChecker()

	log.Printf("[SERVER] 日志管理: http://localhost%s", listenAddr)
	log.Printf("ClickHouse: %s", chURL)
	log.Fatal(http.ListenAndServe(listenAddr, http.HandlerFunc(serverRouter)))
}
