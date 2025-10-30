package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SeverityLevel represents OpenTelemetry log severity levels
type SeverityLevel string

const (
	SeverityTrace SeverityLevel = "TRACE"
	SeverityDebug SeverityLevel = "DEBUG"
	SeverityInfo  SeverityLevel = "INFO"
	SeverityWarn  SeverityLevel = "WARN"
	SeverityError SeverityLevel = "ERROR"
	SeverityFatal SeverityLevel = "FATAL"
)

// LogRecord represents an OpenTelemetry-compliant log record
type LogRecord struct {
	Timestamp         string                 `json:"timestamp"`
	ObservedTimestamp string                 `json:"observed_timestamp"`
	SeverityText      string                 `json:"severity_text"`
	SeverityNumber    int                    `json:"severity_number"`
	Body              string                 `json:"body"`
	Attributes        map[string]interface{} `json:"attributes"`
}

// Logger provides structured logging
type Logger struct {
	enabled bool
}

var defaultLogger = &Logger{enabled: true}

// SetEnabled controls whether logging is enabled
func SetEnabled(enabled bool) {
	defaultLogger.enabled = enabled
}

func severityToNumber(severity SeverityLevel) int {
	switch severity {
	case SeverityTrace:
		return 1
	case SeverityDebug:
		return 5
	case SeverityInfo:
		return 9
	case SeverityWarn:
		return 13
	case SeverityError:
		return 17
	case SeverityFatal:
		return 21
	default:
		return 0
	}
}

func (l *Logger) log(severity SeverityLevel, body string, attributes map[string]interface{}) {
	if !l.enabled {
		return
	}

	now := time.Now()
	record := LogRecord{
		Timestamp:         now.Format(time.RFC3339Nano),
		ObservedTimestamp: now.Format(time.RFC3339Nano),
		SeverityText:      string(severity),
		SeverityNumber:    severityToNumber(severity),
		Body:              body,
		Attributes:        attributes,
	}

	data, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log record: %v\n", err)
		return
	}

	fmt.Println(string(data))
}

// RPCRequestReceived logs when an RPC request is received
func RPCRequestReceived(method string, attributes map[string]interface{}) {
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["rpc.method"] = method
	attributes["event.name"] = "rpc.request.received"
	defaultLogger.log(SeverityInfo, "RPC request received", attributes)
}

// RPCResponseSent logs when an RPC response is sent
func RPCResponseSent(method string, success bool, attributes map[string]interface{}) {
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["rpc.method"] = method
	attributes["rpc.success"] = success
	attributes["event.name"] = "rpc.response.sent"
	defaultLogger.log(SeverityInfo, "RPC response sent", attributes)
}

// BackendMethodStart logs when a backend method starts
func BackendMethodStart(method string, attributes map[string]interface{}) {
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["backend.method"] = method
	attributes["event.name"] = "backend.method.start"
	defaultLogger.log(SeverityDebug, "Backend method started", attributes)
}

// BackendMethodEnd logs when a backend method ends
func BackendMethodEnd(method string, success bool, attributes map[string]interface{}) {
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["backend.method"] = method
	attributes["backend.success"] = success
	attributes["event.name"] = "backend.method.end"
	defaultLogger.log(SeverityDebug, "Backend method ended", attributes)
}

// EventReceived logs when a CDP event is received
func EventReceived(eventType string, attributes map[string]interface{}) {
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["cdp.event.type"] = eventType
	attributes["event.name"] = "cdp.event.received"
	defaultLogger.log(SeverityDebug, "CDP event received", attributes)
}

// Info logs an informational message
func Info(body string, attributes map[string]interface{}) {
	defaultLogger.log(SeverityInfo, body, attributes)
}

// Debug logs a debug message
func Debug(body string, attributes map[string]interface{}) {
	defaultLogger.log(SeverityDebug, body, attributes)
}

// Warn logs a warning message
func Warn(body string, attributes map[string]interface{}) {
	defaultLogger.log(SeverityWarn, body, attributes)
}

// Error logs an error message
func Error(body string, attributes map[string]interface{}) {
	defaultLogger.log(SeverityError, body, attributes)
}
