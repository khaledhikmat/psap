package utils

import (
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	entry   T
	created time.Time
}

type Cache[T any] struct {
	timeToLive float64
	data       map[string]cacheEntry[T]
	mutex      sync.Mutex
}

func NewCache[T any](ttl float64) *Cache[T] {
	c := Cache[T]{}
	c.timeToLive = ttl
	c.data = make(map[string]cacheEntry[T])
	return &c
}

func (c *Cache[T]) Set(key string, value T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = cacheEntry[T]{entry: value, created: time.Now()}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry, ok := c.data[key]
	if !ok {
		var zero T
		return zero, false
	}
	if time.Since(entry.created).Seconds() > c.timeToLive {
		delete(c.data, key)
		var zero T
		return zero, false
	}
	return entry.entry, true
}
