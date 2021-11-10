package clog

import (
	"context"
)

// The logger to use for this application
var logger loggerOps = newZapLogger(true) //nolint

// loggerActions defines the available logging actions a logger needs to support
type loggerActions interface {
	Infocf(ctx context.Context, msg string, args ...interface{})
	Warncf(ctx context.Context, msg string, args ...interface{})
	Errorcf(ctx context.Context, msg string, args ...interface{})
}

// loggerOps defines the operations a logger needs to support to be able to use clog
type loggerOps interface {
	loggerActions
	Flush()
}

// EnableColor initializes a new logger with color enabled (not thread safe)
func EnableColor() {
	logger = newZapLogger(true)
}

// EnableColor initializes a new logger with color disabled (not thread safe)
func DisableColor() {
	logger = newZapLogger(false)
}

// Info records a information log using the default logger
func Info(msg string) {
	logger.Infocf(context.Background(), msg)
}

// Infoc records a information log including log fields provided in the context using the default logger
func Infoc(ctx context.Context, msg string) {
	logger.Infocf(ctx, msg)
}

// Infof records a information log using the default logger via a formatted string
func Infof(msg string, args ...interface{}) {
	logger.Infocf(context.Background(), msg, args...)
}

// Infocf records a information log including log fields provided in the context using the default logger via a formatted string
func Infocf(ctx context.Context, msg string, args ...interface{}) {
	logger.Infocf(ctx, msg, args...)
}

// Warn records a warning log using the default logger
func Warn(msg string) {
	logger.Warncf(context.Background(), msg)
}

// Warnc records a warning log including log fields provided in the context using the default logger
func Warnc(ctx context.Context, msg string) {
	logger.Warncf(ctx, msg)
}

// Warnf records a warning log using the default logger via a formatted string
func Warnf(msg string, args ...interface{}) {
	logger.Warncf(context.Background(), msg, args...)
}

// Warncf records a warning log including log fields provided in the context using the default logger via a formatted string
func Warncf(ctx context.Context, msg string, args ...interface{}) {
	logger.Warncf(ctx, msg, args...)
}

// Error records a error log using the default logger
func Error(msg string) {
	logger.Errorcf(context.Background(), msg)
}

// Errorc records a error log including log fields provided in the context using the default logger
func Errorc(ctx context.Context, msg string) {
	logger.Errorcf(ctx, msg)
}

// Errorf records a error log using the default logger via a formatted string
func Errorf(msg string, args ...interface{}) {
	logger.Errorcf(context.Background(), msg, args...)
}

// Errorcf records a error log using the default logger
func Errorcf(ctx context.Context, msg string, args ...interface{}) {
	logger.Errorcf(ctx, msg, args...)
}

// Flush will ensure any logs in transit are written. Should be called when the app is shutting down
func Flush() {
	logger.Flush()
}

// Defines the logging key used to store values in the context that should be logged
type logKey struct{}

// Fields define the values to add to the logs for a specific log entry
type Fields map[string]interface{}

// WithField will create a context with the provided key, value pair appended to the fields stored in the context
func WithField(ctx context.Context, key string, value interface{}) context.Context {
	logFieldsRaw := ctx.Value(logKey{})
	logFields, ok := logFieldsRaw.(Fields)

	if logFields == nil || !ok {
		// We don't recognized the fields in the current context, or it hasn't been set, create a new context with new fields
		return context.WithValue(ctx, logKey{}, Fields{key: value})
	}

	// Otherwise, create a new fields entry for the new context based on the old one and the new key value pair
	newFields := make(Fields)
	for k, v := range logFields {
		newFields[k] = v
	}
	newFields[key] = value
	return context.WithValue(ctx, logKey{}, newFields)
}

// FieldsFromContext extracts the current fields stored in the context
func FieldsFromContext(ctx context.Context) Fields {
	if ctx == nil {
		return nil
	}
	val, _ := ctx.Value(logKey{}).(Fields)
	return val
}
