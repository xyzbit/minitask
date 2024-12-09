package cache

import (
	"sync"
)

type ThreadSafeMap[T any] struct {
	lock  sync.RWMutex
	items map[string]T
}

func NewThreadSafeMap[T any]() *ThreadSafeMap[T] {
	return &ThreadSafeMap[T]{
		items: make(map[string]T),
	}
}

func (c *ThreadSafeMap[T]) Add(key string, obj T) {
	c.Update(key, obj)
}

func (c *ThreadSafeMap[T]) Update(key string, obj T) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items[key] = obj
}

func (c *ThreadSafeMap[T]) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.items, key)
}

func (c *ThreadSafeMap[T]) Get(key string) (item T, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
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

func (c *ThreadSafeMap[T]) ListKeys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]string, 0, len(c.items))
	for key := range c.items {
		list = append(list, key)
	}
	return list
}
