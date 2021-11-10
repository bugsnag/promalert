package clog

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapLogger struct {
	logger *zap.SugaredLogger
}

// utcFormat defines a formatter that will output the date in UTC RFC339 format
func utcFormat() zapcore.TimeEncoder {
	return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.RFC3339))
	}
}

// newZapLogger creates a new logger
func newZapLogger(colorOn bool) *zapLogger {
	cfg := zap.NewProductionConfig()
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = utcFormat()
	if colorOn {
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return &zapLogger{logger: logger.Sugar()}
}

// convertToZapFields will convert fields stored in the context to zap compatible fields
func convertToZapFields(f Fields) []interface{} {
	fields := make([]interface{}, 0, len(f)*2)
	for key, val := range f {
		fields = append(fields, key, val)
	}
	return fields
}

// Infocf records a information log including log fields provided in the context using the logrus logger via a formatted string
func (z *zapLogger) Infocf(ctx context.Context, msg string, args ...interface{}) {
	fields := convertToZapFields(FieldsFromContext(ctx))
	z.logger.Infow(fmt.Sprintf(msg, args...), fields...)
}

// Warncf records a warning log including log fields provided in the context using the logrus logger via a formatted string
func (z *zapLogger) Warncf(ctx context.Context, msg string, args ...interface{}) {
	fields := convertToZapFields(FieldsFromContext(ctx))
	z.logger.Warnw(fmt.Sprintf(msg, args...), fields...)
}

// Errorcf records a error log using the logrus logger
func (z *zapLogger) Errorcf(ctx context.Context, msg string, args ...interface{}) {
	fields := convertToZapFields(FieldsFromContext(ctx))
	z.logger.Errorw(fmt.Sprintf(msg, args...), fields...)
}

// Flush is used to ensure any inflight logs are written
func (z *zapLogger) Flush() {
	_ = z.logger.Sync()
}
