package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Init initialises the global logger
func Init(env string) {
	var cfg zap.Config

	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var err error
	log, err = cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic("failed to initialise logger: " + err.Error())
	}
}

// Get returns the global logger instance
func Get() *zap.Logger {
	if log == nil {
		// Fallback to a basic logger if not initialised
		log, _ = zap.NewDevelopment()
	}
	return log
}

// WithService returns a logger with a service name field
func WithService(service string) *zap.Logger {
	return Get().With(zap.String("service", service))
}

// WithRequestID returns a logger with a request ID field
func WithRequestID(requestID string) *zap.Logger {
	return Get().With(zap.String("request_id", requestID))
}

// Sync flushes any buffered log entries
func Sync() {
	if log != nil {
		_ = log.Sync()
	}
}

// Convenience wrappers
func Info(msg string, fields ...zap.Field)  { Get().Info(msg, fields...) }
func Error(msg string, fields ...zap.Field) { Get().Error(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { Get().Warn(msg, fields...) }
func Debug(msg string, fields ...zap.Field) { Get().Debug(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
	os.Exit(1)
}
