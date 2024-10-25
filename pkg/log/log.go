package log

import (
	"sync"

	"go.uber.org/zap"
)

var (
	_globalMu     sync.RWMutex
	_globalLogger Logger = &DefaultLogger{zap.L().Sugar()}
)

type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Panic(args ...interface{})
	Fatal(args ...interface{})
}

func Debug(args ...interface{}) {
	Global().Debug(args...)
}

func Info(args ...interface{}) {
	Global().Info(args...)
}

func Warn(args ...interface{}) {
	Global().Warn(args...)
}

func Error(args ...interface{}) {
	Global().Error(args...)
}

func Panic(args ...interface{}) {
	Global().Panic(args...)
}

func Faltal(args ...interface{}) {
	Global().Fatal(args...)
}

func Global() Logger {
	_globalMu.RLock()
	defer _globalMu.RUnlock()
	return _globalLogger
}

func ReplaceGlobal(l Logger) {
	_globalMu.Lock()
	defer _globalMu.Unlock()
	_globalLogger = l
}

func NewLoggerByzap(z *zap.SugaredLogger) Logger {
	return &DefaultLogger{SugaredLogger: z}
}

type DefaultLogger struct {
	*zap.SugaredLogger
}

func (l *DefaultLogger) Debug(args ...interface{}) {
	if l == nil {
		return
	}
	if len(args) == 0 {
		return
	}
	if len(args) == 1 {
		l.SugaredLogger.Debug(args[0])
		return
	}
	l.SugaredLogger.Debugf(args[0].(string), args[1:]...)
}

func (l *DefaultLogger) Info(args ...interface{}) {
	if l == nil {
		return
	}
	if len(args) == 0 {
		return
	}
	if len(args) == 1 {
		l.SugaredLogger.Info(args[0])
		return
	}
	l.SugaredLogger.Infof(args[0].(string), args[1:]...)
}

func (l *DefaultLogger) Warn(args ...interface{}) {
	if l == nil {
		return
	}
	if len(args) == 0 {
		return
	}
	if len(args) == 1 {
		l.SugaredLogger.Warn(args[0])
		return
	}
	l.SugaredLogger.Warnf(args[0].(string), args[1:]...)
}

func (l *DefaultLogger) Error(args ...interface{}) {
	if l == nil {
		return
	}
	if len(args) == 0 {
		return
	}
	if len(args) == 1 {
		l.SugaredLogger.Error(args[0])
		return
	}
	l.SugaredLogger.Errorf(args[0].(string), args[1:]...)
}

func (l *DefaultLogger) Panic(args ...interface{}) {
	if l == nil {
		return
	}
	if len(args) == 0 {
		return
	}
	if len(args) == 1 {
		l.SugaredLogger.Panic(args[0])
		return
	}
	l.SugaredLogger.Panicf(args[0].(string), args[1:]...)
}

func (l *DefaultLogger) Fatal(args ...interface{}) {
	if l == nil {
		return
	}
	if len(args) == 0 {
		return
	}
	if len(args) == 1 {
		l.SugaredLogger.Fatal(args[0])
		return
	}
	l.SugaredLogger.Fatalf(args[0].(string), args[1:]...)
}
