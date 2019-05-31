package logging

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

// Logger - contains all the specifics for the logger instance
type Logger struct {

	// internal, specific to the underlying logging library
	contextData log.Fields

	// internal, specific to the underlying logging library
	logger log.Logger

	// independent of the underlying logging library
	LogInterface
}

// DataFields - shorthand for a custom type
type DataFields map[string]interface{}

// LogInterface - exposes the top level interface for logger package
type LogInterface interface {
	Debug(message string, data DataFields)
	Info(message string, data DataFields)
	Warn(message string, data DataFields)
	Error(message string, data DataFields)
}

// LogConfig - a struct to contain all the specifics for a logger
type LogConfig struct {
	AppName     string
	AppVersion  string
	EngGroup    string
	Environment string // example: development, staging & prod
	Level       string // example: DEBUG, INFO, WARN, ERROR
}

// New - instantiates a new Logger struct with necessary details
func New(config *LogConfig) (Logger, error) {
	// parse through level
	var logLevel log.Level

	switch config.Level {
	case "DEBUG", "debug":
		logLevel = log.DebugLevel
	case "INFO", "info":
		logLevel = log.InfoLevel
	case "WARN", "warn":
		logLevel = log.WarnLevel
	case "ERROR", "error":
		logLevel = log.ErrorLevel
	case "PANIC", "panic":
		fallthrough
	case "FATAL", "fatal":
		fallthrough
	default:
		return Logger{}, fmt.Errorf("Invalid logging level. valid options are DEBUG, INFO, WARN & ERROR")
	}

	// TODO: Add checker for environment
	return Logger{contextData: log.Fields{
		"app-name":    config.AppName,
		"app-version": config.AppVersion,
		"eng-group":   config.EngGroup,
		"env":         config.Environment,
	}, logger: log.Logger{
		Out:       os.Stdout,
		Formatter: new(log.JSONFormatter),
		Level:     logLevel,
	}}, nil
}

// Debug - logs message at the 'debug' level
func (l *Logger) Debug(message string, data ...DataFields) {
	if data == nil {
		l.logger.WithFields(l.contextData).Debug(message)
		return
	}

	newData := l.mapJoin(data[0])
	l.logger.WithFields(newData).Debug(message)
}

// Info - logs message at the 'info' level
func (l *Logger) Info(message string, data ...DataFields) {
	if data == nil {
		l.logger.WithFields(l.contextData).Info(message)
		return
	}

	newData := l.mapJoin(data[0])
	l.logger.WithFields(newData).Info(message)
}

// Warn - logs message at the 'warn' level.
func (l *Logger) Warn(message string, data ...DataFields) {
	if data == nil {
		l.logger.WithFields(l.contextData).Warn(message)
		return
	}

	newData := l.mapJoin(data[0])
	l.logger.WithFields(newData).Warn(message)
}

// Error - logs messages at the error level
func (l *Logger) Error(message string, data ...DataFields) {
	if data == nil {
		l.logger.WithFields(l.contextData).Error(message)
		return
	}

	newData := l.mapJoin(data[0])
	l.logger.WithFields(newData).Error(message)
}

// private
func (l *Logger) mapJoin(fieldData DataFields) log.Fields {
	fields := make(log.Fields, len(fieldData)+len(l.contextData))

	for k, v := range fieldData {
		fields[k] = v
	}

	for k, v := range l.contextData {
		fields[k] = v
	}
	return fields
}
