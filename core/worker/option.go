package worker

import (
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
)

type options struct {
	reportResourceInterval time.Duration
	// forced triggering of full task status comparison
	resync time.Duration

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

func WithTriggerResync(interval time.Duration) Option {
	return func(o *options) {
		o.resync = interval
	}
}

func newOptions(opts ...Option) *options {
	// set default
	o := options{
		logger:                 log.Global(),
		reportResourceInterval: 10 * time.Second,
		resync:                 15 * time.Second,
		shutdownTimeout:        180 * time.Second,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &o
}
