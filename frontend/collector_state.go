package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
)

type CollectorState struct {
	CollectorID string `json:"Collector_ID"`
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	Address     string `json:"Address"`
	SourceHost  string `json:"Source_Host"`
}

func getLocalSourceHost() string {
	host, _ := os.Hostname()
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return host + " (" + ipnet.IP.String() + ")"
			}
		}
	}
	return host
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
		return CollectorState{}
	}
	var loaded CollectorState
	if json.Unmarshal(data, &loaded) == nil {
		// 保护：ID 为空时视为未注册
		if loaded.CollectorID == "" {
			return CollectorState{}
		}
		return loaded
	}
	return CollectorState{}
}

func saveCollectorState(cs CollectorState) {
	os.MkdirAll(filepath.Dir(collectorStateFile), 0755)
	data, _ := json.MarshalIndent(cs, "", "  ")
	os.WriteFile(collectorStateFile, data, 0644)
}
