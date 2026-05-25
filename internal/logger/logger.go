package logger

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"
)

type Logger struct {
	service string
	logger  *log.Logger
}

type Fields map[string]any

func New(service string) Logger {
	return NewWithWriter(service, os.Stdout)
}

func NewWithWriter(service string, writer io.Writer) Logger {
	return Logger{
		service: service,
		logger:  log.New(writer, "", 0),
	}
}

func (l Logger) Info(message string, fields Fields) {
	l.write("info", message, fields)
}

func (l Logger) Error(message string, err error, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.write("error", message, fields)
}

func (l Logger) Fatal(message string, err error, fields Fields) {
	l.Error(message, err, fields)
	os.Exit(1)
}

func (l Logger) write(level string, message string, fields Fields) {
	record := Fields{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"level":   level,
		"service": l.service,
		"message": message,
	}
	for key, value := range fields {
		if value != nil {
			record[key] = value
		}
	}

	raw, err := json.Marshal(record)
	if err != nil {
		l.logger.Printf(`{"level":"error","service":%q,"message":"marshal log record","error":%q}`, l.service, err.Error())
		return
	}
	l.logger.Print(string(raw))
}
