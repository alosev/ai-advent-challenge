package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ResponseRecord contains only a problematic HTTP response and its timestamp.
type ResponseRecord struct {
	Time       time.Time `json:"time"`
	HTTPStatus int       `json:"http_status"`
	Error      string    `json:"error"`
	Body       string    `json:"body"`
}

type ResponseLogger struct {
	mu   sync.Mutex
	file *os.File
}

func NewResponseLogger(path string) (*ResponseLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return nil, err
	}
	return &ResponseLogger{file: file}, nil
}

func (l *ResponseLogger) Write(record ResponseRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return json.NewEncoder(l.file).Encode(record)
}

func (l *ResponseLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
