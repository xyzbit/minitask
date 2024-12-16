package worker

import (
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
)

type options struct {
	reportResourceInterval time.Duration
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
		runInterval:            1 * time.Second,
		shutdownTimeout:        180 * time.Second,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &o
}
