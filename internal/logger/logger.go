package logger

import (
	"log"
	"os"
)

// Logger interface for structured logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, err error, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Fatal(msg string, err error, fields ...interface{})
}

// SimpleLogger implements Logger with basic Go logging
type SimpleLogger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	warnLogger  *log.Logger
	debugLogger *log.Logger
}

// NewSimpleLogger creates a new simple logger
func NewSimpleLogger() Logger {
	return &SimpleLogger{
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		warnLogger:  log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile),
		debugLogger: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

// Info logs an info message
func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.infoLogger.Printf("%s %v", msg, fields)
	} else {
		l.infoLogger.Print(msg)
	}
}

// Error logs an error message
func (l *SimpleLogger) Error(msg string, err error, fields ...interface{}) {
	if len(fields) > 0 {
		l.errorLogger.Printf("%s: %v %v", msg, err, fields)
	} else {
		l.errorLogger.Printf("%s: %v", msg, err)
	}
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.warnLogger.Printf("%s %v", msg, fields)
	} else {
		l.warnLogger.Print(msg)
	}
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.debugLogger.Printf("%s %v", msg, fields)
	} else {
		l.debugLogger.Print(msg)
	}
}

// Fatal logs a fatal error and exits
func (l *SimpleLogger) Fatal(msg string, err error, fields ...interface{}) {
	if len(fields) > 0 {
		l.errorLogger.Fatalf("%s: %v %v", msg, err, fields)
	} else {
		l.errorLogger.Fatalf("%s: %v", msg, err)
	}
}