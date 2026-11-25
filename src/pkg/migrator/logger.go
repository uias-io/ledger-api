package migrator

import (
	"log/slog"
)

// Logger defines the logging interface for migration operations
type Logger interface {
	Debugf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

type SlogAdapter struct {
	logger *slog.Logger
}

func NewSlogAdapter(l *slog.Logger) Logger {
	return &SlogAdapter{logger: l}
}

func (s *SlogAdapter) Debugf(format string, v ...interface{}) {
	s.logger.Debug(format, v...)
}

func (s *SlogAdapter) Infof(format string, v ...interface{}) {
	s.logger.Info(format, v...)
}

func (s *SlogAdapter) Warnf(format string, v ...interface{}) {
	s.logger.Warn(format, v...)
}

func (s *SlogAdapter) Errorf(format string, v ...interface{}) {
	s.logger.Error(format, v...)
}
