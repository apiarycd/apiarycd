package badgerfx

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

type zapLogger struct {
	logger *zap.Logger
}

func newLogger(l *zap.Logger) *zapLogger {
	return &zapLogger{
		logger: l,
	}
}

// Debugf implements badger.Logger.
func (l *zapLogger) Debugf(format string, a ...any) {
	l.logger.Debug(fmt.Sprintf(format, a...))
}

// Errorf implements badger.Logger.
func (l *zapLogger) Errorf(format string, a ...any) {
	l.logger.Error(fmt.Sprintf(format, a...))
}

// Infof implements badger.Logger.
func (l *zapLogger) Infof(format string, a ...any) {
	l.logger.Info(fmt.Sprintf(format, a...))
}

// Warningf implements badger.Logger.
func (l *zapLogger) Warningf(format string, a ...any) {
	l.logger.Warn(fmt.Sprintf(format, a...))
}

var _ badger.Logger = (*zapLogger)(nil)
