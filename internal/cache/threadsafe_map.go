package cache

import (
	"sync"
)

type ThreadSafeMap struct {
	lock  sync.RWMutex
	items map[string]interface{}
}

func NewThreadSafeMap() *ThreadSafeMap {
	return &ThreadSafeMap{
		items: make(map[string]interface{}),
	}
}

func (c *ThreadSafeMap) Add(key string, obj interface{}) {
	c.Update(key, obj)
}

func (c *ThreadSafeMap) Update(key string, obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items[key] = obj
}

func (c *ThreadSafeMap) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, exists := c.items[key]; exists {
		delete(c.items, key)
	}
}

func (c *ThreadSafeMap) Get(key string) (item interface{}, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
}

func (c *ThreadSafeMap) List() []interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]interface{}, 0, len(c.items))
	for _, item := range c.items {
		list = append(list, item)
	}
	return list
}

func (c *ThreadSafeMap) ListKeys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]string, 0, len(c.items))
	for key := range c.items {
		list = append(list, key)
	}
	return list
}
