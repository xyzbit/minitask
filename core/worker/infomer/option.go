package infomer

import (
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
)

type options struct {
	reportResourceInterval time.Duration
	syncRealStatusInterval time.Duration
	runInterval            time.Duration

	shutdownTimeout time.Duration
	logger          log.Logger
}

type Option func(o *options)

func WithLogger(logger log.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.shutdownTimeout = timeout
	}
}

func WithReportResourceInterval(interval time.Duration) Option {
	return func(o *options) {
		o.reportResourceInterval = interval
	}
}

func WithSyncRealStatusInterval(interval time.Duration) Option {
	return func(o *options) {
		o.syncRealStatusInterval = interval
	}
}

func WithRunInterval(interval time.Duration) Option {
	return func(o *options) {
		o.runInterval = interval
	}
}

func newOptions(opts ...Option) *options {
	// set default
	o := options{
		logger:                 log.Global(),
		reportResourceInterval: 10 * time.Second,
		syncRealStatusInterval: 2 * time.Second,
		runInterval:            1 * time.Second,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &o
}
