package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const chunkSize = 2 * 1024 * 1024 // 2MB read buffer

func main() {
	logDir := "/home/yxp/LMS_mimo/collector/elk_logs/incoming/"
	if len(os.Args) > 1 {
		logDir = os.Args[1]
	}
	if !strings.HasSuffix(logDir, "/") {
		logDir += "/"
	}

	info, err := os.Stat(logDir)
	if err != nil || !info.IsDir() {
		return
	}

	exeDir := filepath.Dir(os.Args[0])
	stateFile := filepath.Join(exeDir, ".processed.json")
	lockFile := filepath.Join(exeDir, ".reader.lock")

	// 防止重叠执行（锁超过 10 分钟自动失效）
	if info, err := os.Stat(lockFile); err == nil {
		if time.Since(info.ModTime()) < 10*time.Minute {
			return
		}
		os.Remove(lockFile)
	}
	os.WriteFile(lockFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(lockFile)

	state := loadState(stateFile)

	files, _ := filepath.Glob(logDir + "*.json")
	sort.Strings(files)

	for _, fp := range files {
		fname := filepath.Base(fp)
		finfo, err := os.Stat(fp)
		if err != nil {
			continue
		}
		fkey := fmt.Sprintf("%s:%d", fname, finfo.Size())

		if state[fkey] {
			continue
		}

		processFile(fp)
		state[fkey] = true
	}

	// 清理已删除文件的状态
	existingFiles, _ := filepath.Glob(logDir + "*.json")
	existing := make(map[string]bool)
	for _, f := range existingFiles {
		existing[filepath.Base(f)] = true
	}
	for k := range state {
		name := strings.SplitN(k, ":", 2)[0]
		if !existing[name] {
			delete(state, k)
		}
	}

	saveState(stateFile, state)
}

func loadState(path string) map[string]bool {
	state := make(map[string]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	var raw map[string]interface{}
	if json.Unmarshal(data, &raw) == nil {
		for k, v := range raw {
			if b, ok := v.(bool); ok && b {
				state[k] = true
			}
		}
	}
	return state
}

func saveState(path string, state map[string]bool) {
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(path, data, 0644)
}

func processFile(filepath string) {
	f, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer f.Close()

	dec := json.NewDecoder(f)

	// 跳过开头的 '['
	t, err := dec.Token()
	if err != nil {
		return
	}
	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		// 不是数组 — 回退处理：整个文件当成单个对象或 NDJSON
		f.Seek(0, 0)
		processFallback(f)
		return
	}

	// 逐个解析数组中的对象并输出 NDJSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	for dec.More() {
		var obj json.RawMessage
		if err := dec.Decode(&obj); err != nil {
			// 跳过损坏的对象，继续尝试
			continue
		}
		os.Stdout.Write(obj)
		os.Stdout.Write([]byte("\n"))
	}
}

// processFallback 处理非数组格式（单对象或逐行 JSON）
func processFallback(f *os.File) {
	// 先尝试逐行解析（NDJSON）
	scanner := json.NewDecoder(f)
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	for {
		var obj json.RawMessage
		if err := scanner.Decode(&obj); err == io.EOF {
			break
		} else if err != nil {
			break
		}
		os.Stdout.Write(obj)
		os.Stdout.Write([]byte("\n"))
	}
}
