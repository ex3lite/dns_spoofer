package proxy

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

var (
	debugLogPath   string
	debugLogPathOk bool
	debugLogOnce   sync.Once
)

func getDebugLogPath() string {
	debugLogOnce.Do(func() {
		debugLogPath = os.Getenv("DEBUG_LOG_PATH")
		debugLogPathOk = debugLogPath != ""
	})
	if !debugLogPathOk {
		return ""
	}
	return debugLogPath
}

// debugLog appends one NDJSON line to DEBUG_LOG_PATH when set. Payload gets id, timestamp, location, message, data, hypothesisId.
func debugLog(location, message string, hypothesisIDs []string, data map[string]interface{}) {
	path := getDebugLogPath()
	if path == "" {
		return
	}
	if data == nil {
		data = make(map[string]interface{})
	}
	payload := map[string]interface{}{
		"id":        "log_proxy",
		"timestamp": time.Now().UnixMilli(),
		"location":  location,
		"message":   message,
		"data":      data,
	}
	if len(hypothesisIDs) > 0 {
		payload["hypothesisId"] = hypothesisIDs[0]
	}
	line, _ := json.Marshal(payload)
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	_, _ = f.Write(line)
	_ = f.Close()
}
