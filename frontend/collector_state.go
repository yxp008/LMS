package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type CollectorState struct {
	CollectorID string `json:"Collector_ID"`
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	Address     string `json:"Address"`
}

func generateCollectorID() string {
	b := make([]byte, 4)
	rand.Read(b)
	host, _ := os.Hostname()
	if len(host) > 6 { host = host[:6] }
	return "C_" + host + "_" + hex.EncodeToString(b)
}

func loadCollectorState() CollectorState {
	data, err := os.ReadFile(collectorStateFile)
	if err != nil {
		// 首次运行：自动生成唯一ID
		cs := CollectorState{CollectorID: generateCollectorID(), Name: "Vector-WSL", Status: "1", Address: ""}
		saveCollectorState(cs)
		return cs
	}
	var loaded CollectorState
	if json.Unmarshal(data, &loaded) == nil {
		return loaded
	}
	cs := CollectorState{CollectorID: generateCollectorID(), Name: "Vector-WSL", Status: "1", Address: ""}
	saveCollectorState(cs)
	return cs
}

func saveCollectorState(cs CollectorState) {
	os.MkdirAll(filepath.Dir(collectorStateFile), 0755)
	data, _ := json.MarshalIndent(cs, "", "  ")
	os.WriteFile(collectorStateFile, data, 0644)
}
