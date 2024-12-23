package cache

import (
	"sync"
	"time"
)

const DefaultRecycleInterval = 1 * time.Minute

type ThreadSafeMap[T any] struct {
	lock    sync.RWMutex
	setTime map[string]time.Time
	items   map[string]T
}

func NewThreadSafeMap[T any](condition func(item T, afterSetDurtion time.Duration) bool) *ThreadSafeMap[T] {
	tsm := &ThreadSafeMap[T]{
		items:   make(map[string]T),
		setTime: make(map[string]time.Time),
	}

	go func() {
		for {
			time.Sleep(DefaultRecycleInterval)
			for key, item := range tsm.listWithSetDurition() {
				if condition(item.item, item.d) {
					tsm.Delete(key)
				}
			}
		}
	}()

	return tsm
}

func (c *ThreadSafeMap[T]) Set(key string, obj T) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items[key] = obj
	c.setTime[key] = time.Now()
}

func (c *ThreadSafeMap[T]) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.items, key)
	delete(c.setTime, key)
}

func (c *ThreadSafeMap[T]) Get(key string) (item T, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
}

type itemWithDurition[T any] struct {
	item T
	d    time.Duration
}

func (c *ThreadSafeMap[T]) listWithSetDurition() map[string]itemWithDurition[T] {
	c.lock.RLock()
	defer c.lock.RUnlock()
	m := make(map[string]itemWithDurition[T], len(c.items))
	for key, item := range c.items {
		setTime := c.setTime[key]
		m[key] = itemWithDurition[T]{
			item: item,
			d:    time.Now().Sub(setTime),
		}
	}
	return m
}

func (c *ThreadSafeMap[T]) List() []T {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]T, 0, len(c.items))
	for _, item := range c.items {
		list = append(list, item)
	}
	return list
}
