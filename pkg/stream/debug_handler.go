package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HandleDebugLog receives debug logs from the Cast receiver
func (p *RemoteHandler) HandleDebugLog(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var logData map[string]interface{}
	if err := json.Unmarshal(body, &logData); err != nil {
		// If not JSON, just print raw text
		fmt.Printf("📺 [RECEIVER] %s\n", string(body))
	} else {
		// Pretty print the log
		level := logData["level"]
		message := logData["message"]
		tag := logData["tag"]

		levelEmoji := "📺"
		switch level {
		case "ERROR":
			levelEmoji = "❌"
		case "WARN":
			levelEmoji = "⚠️"
		case "INFO":
			levelEmoji = "ℹ️"
		case "DEBUG":
			levelEmoji = "🔍"
		}

		if tag != nil {
			fmt.Printf("%s [%v] [%v] %v\n", levelEmoji, level, tag, message)
		} else {
			fmt.Printf("%s [%v] %v\n", levelEmoji, level, message)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
