package main

import (
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

func loadCollectorState() CollectorState {
	cs := CollectorState{CollectorID: "C001", Name: "Vector-WSL", Status: "1", Address: ""}
	data, err := os.ReadFile(collectorStateFile)
	if err != nil {
		return cs
	}
	var loaded CollectorState
	if json.Unmarshal(data, &loaded) == nil {
		return loaded
	}
	return cs
}

func saveCollectorState(cs CollectorState) {
	os.MkdirAll(filepath.Dir(collectorStateFile), 0755)
	data, _ := json.MarshalIndent(cs, "", "  ")
	os.WriteFile(collectorStateFile, data, 0644)
}
