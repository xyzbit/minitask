package queue

import (
	"sync"
	"time"

	"github.com/xyzbit/minitaskx/internal/clock"
)

// TypedInterface warp a queue to supports advanced queue functions
// such as concurrency control and Remove duplicates judgment
type TypedInterface[T comparable] interface {
	Add(item T) (exist bool)
	Len() int
	Get() (item T, shutdown bool)
	Done(item T)
	ShutDown()
	ShutDownWithDrain()
	ShuttingDown() bool
}

// UniKeyable is an interface of TypedInterface'Item through which you can customize the comparison logic of elements.
// Note that this T and TypedInterface T need to be of the same type, and don't use pointer types.
// for this way, we can handle the situation where some fields in the structure are used as keys.
// eg.
//
//	type MyTask struct {
//	  key string `json:"key"`
//		 name string `json:"name"`
//	}
//
// func (t MyTask) GetUniKey() MyTask {return MyTask{key: t.key}}
type UniKey[T comparable] interface {
	GetUniKey() T
}

// Queue is the underlying storage for items. The functions below are always
// called from the same goroutine.
type Queue[T comparable] interface {
	// Touch can be hooked when an existing item is added again. This may be
	// useful if the implementation allows priority change for the given item.
	Touch(item T)
	// Push adds a new item.
	Push(item T)
	// Len tells the total number of items.
	Len() int
	// Pop retrieves an item.
	Pop() (item T)
}

// DefaultQueue is a slice based FIFO queue.
func DefaultQueue[T comparable]() Queue[T] {
	return new(queue[T])
}

// queue is a slice which implements Queue.
type queue[T comparable] []T

func (q *queue[T]) Touch(item T) {}

func (q *queue[T]) Push(item T) {
	*q = append(*q, item)
}

func (q *queue[T]) Len() int {
	return len(*q)
}

func (q *queue[T]) Pop() (item T) {
	item = (*q)[0]

	// The underlying array still exists and reference this object, so the object will not be garbage collected.
	(*q)[0] = *new(T)
	*q = (*q)[1:]

	// When the length is much smaller than the capacity, create a new slice to free up memory.
	if cap(*q) > 256 && len(*q) < cap(*q)/4 {
		newQueue := make(queue[T], len(*q))
		copy(newQueue, *q)
		*q = newQueue
	}

	return item
}

type TypedQueueConfig[T comparable] struct {
	// Name for the queue. If unnamed, the metrics will not be registered.
	Name string

	// MetricsProvider optionally allows specifying a metrics provider to use for the queue
	// instead of the global provider.
	MetricsProvider MetricsProvider

	// Clock ability to inject real or fake clock for testing purposes.
	Clock clock.WithTicker

	// Queue provides the underlying queue to use. It is optional and defaults to slice based FIFO queue.
	Queue Queue[T]
}

// NewTyped constructs a new work queue (see the package comment).
func NewTyped[T comparable]() *Typed[T] {
	return NewTypedWithConfig(TypedQueueConfig[T]{
		Name: "",
	})
}

// NewTypedWithConfig constructs a new workqueue with ability to
// customize different properties.
func NewTypedWithConfig[T comparable](config TypedQueueConfig[T]) *Typed[T] {
	return newQueueWithConfig(config, defaultUnfinishedWorkUpdatePeriod)
}

// newQueueWithConfig constructs a new named workqueue
// with the ability to customize different properties for testing purposes
func newQueueWithConfig[T comparable](config TypedQueueConfig[T], updatePeriod time.Duration) *Typed[T] {
	var metricsFactory *queueMetricsFactory
	if config.MetricsProvider != nil {
		metricsFactory = &queueMetricsFactory{
			metricsProvider: config.MetricsProvider,
		}
	} else {
		metricsFactory = &globalMetricsFactory
	}

	if config.Clock == nil {
		config.Clock = clock.RealClock{}
	}

	if config.Queue == nil {
		config.Queue = DefaultQueue[T]()
	}

	return newQueue(
		config.Clock,
		config.Queue,
		metricsFactory.newQueueMetrics(config.Name, config.Clock),
		updatePeriod,
	)
}

func newQueue[T comparable](c clock.WithTicker, queue Queue[T], metrics queueMetrics, updatePeriod time.Duration) *Typed[T] {
	t := &Typed[T]{
		clock:                      c,
		queue:                      queue,
		dirty:                      set[T]{},
		processing:                 set[T]{},
		cond:                       sync.NewCond(&sync.Mutex{}),
		metrics:                    metrics,
		unfinishedWorkUpdatePeriod: updatePeriod,
	}

	// Don't start the goroutine for a type of noMetrics so we don't consume
	// resources unnecessarily
	if _, ok := metrics.(noMetrics); !ok {
		go t.updateUnfinishedWorkLoop()
	}

	return t
}

const defaultUnfinishedWorkUpdatePeriod = 500 * time.Millisecond

// Type is a work queue (see the package comment).
// Deprecated: Use Typed instead.
type Type = Typed[any]

type Typed[t comparable] struct {
	// queue defines the order in which we will work on items. Every
	// element of queue should be in the dirty set and not in the
	// processing set.
	queue Queue[t]

	// dirty defines all of the items that need to be processed.
	dirty set[t]

	// Things that are currently being processed are in the processing set.
	// These things may be simultaneously in the dirty set. When we finish
	// processing something and remove it from this set, we'll check if
	// it's in the dirty set, and if so, add it to the queue.
	processing set[t]

	cond *sync.Cond

	shuttingDown bool
	drain        bool

	metrics queueMetrics

	unfinishedWorkUpdatePeriod time.Duration
	clock                      clock.WithTicker
}

// Add marks item as needing processing.
func (q *Typed[T]) Add(item T) (exist bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.shuttingDown {
		return false
	}
	if q.dirty.has(item) {
		// the same item is added again before it is processed, call the Touch
		// function if the queue cares about it (for e.g, reset its priority)
		if !q.processing.has(item) {
			q.queue.Touch(item)
		}
		return true
	}

	if q.processing.has(item) {
		return true
	}
	q.metrics.add(item)
	q.dirty.insert(item)

	q.queue.Push(item)
	q.cond.Signal()
	return false
}

// Len returns the current queue length, for informational purposes only. You
// shouldn't e.g. gate a call to Add() or Get() on Len() being a particular
// value, that can't be synchronized properly.
func (q *Typed[T]) Len() int {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	return q.queue.Len()
}

// Get blocks until it can return an item to be processed. If shutdown = true,
// the caller should end their goroutine. You must call Done with item when you
// have finished processing it.
func (q *Typed[T]) Get() (item T, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for q.queue.Len() == 0 && !q.shuttingDown {
		q.cond.Wait()
	}
	if q.queue.Len() == 0 {
		// We must be shutting down.
		return *new(T), true
	}

	item = q.queue.Pop()

	q.metrics.get(item)

	q.processing.insert(item)
	q.dirty.delete(item)

	return item, false
}

// Done marks item as done processing, and if it has been marked as dirty again
// while it was being processed, it will be re-added to the queue for
// re-processing.
func (q *Typed[T]) Done(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.metrics.done(item)

	q.processing.delete(item)
	if q.processing.len() == 0 {
		q.cond.Signal()
	}
}

// ShutDown will cause q to ignore all new items added to it and
// immediately instruct the worker goroutines to exit.
func (q *Typed[T]) ShutDown() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.drain = false
	q.shuttingDown = true
	q.cond.Broadcast()
}

// ShutDownWithDrain will cause q to ignore all new items added to it. As soon
// as the worker goroutines have "drained", i.e: finished processing and called
// Done on all existing items in the queue; they will be instructed to exit and
// ShutDownWithDrain will return. Hence: a strict requirement for using this is;
// your workers must ensure that Done is called on all items in the queue once
// the shut down has been initiated, if that is not the case: this will block
// indefinitely. It is, however, safe to call ShutDown after having called
// ShutDownWithDrain, as to force the queue shut down to terminate immediately
// without waiting for the drainage.
func (q *Typed[T]) ShutDownWithDrain() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.drain = true
	q.shuttingDown = true
	q.cond.Broadcast()

	for q.processing.len() != 0 && q.drain {
		q.cond.Wait()
	}
}

func (q *Typed[T]) ShuttingDown() bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	return q.shuttingDown
}

func (q *Typed[T]) updateUnfinishedWorkLoop() {
	t := q.clock.NewTicker(q.unfinishedWorkUpdatePeriod)
	defer t.Stop()
	for range t.C() {
		if !func() bool {
			q.cond.L.Lock()
			defer q.cond.L.Unlock()
			if !q.shuttingDown {
				q.metrics.updateUnfinishedWork()
				return true
			}
			return false
		}() {
			return
		}
	}
}
