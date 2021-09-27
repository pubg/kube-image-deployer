package util

import (
	"sync"
	"time"
)

type Cache struct {
	TTL uint

	cache map[string]*cacheResult
	mutex *sync.Mutex
}

type cacheResult struct {
	value interface{}
	err   error
	time  time.Time
	mutex *sync.Mutex
}

func NewCache(ttlSeconds uint) *Cache {
	return &Cache{
		TTL:   ttlSeconds,
		cache: make(map[string]*cacheResult),
		mutex: &sync.Mutex{},
	}
}

func (c *Cache) Get(key string, getter func() (interface{}, error)) (interface{}, error) {
	c.mutex.Lock()
	cache, ok := c.cache[key]
	c.mutex.Unlock()

	if ok && time.Since(cache.time) < time.Duration(c.TTL)*time.Second {
		cache.mutex.Lock()
		value, err := cache.value, cache.err
		cache.mutex.Unlock()
		return value, err
	}

	if ok {
		cache.mutex.Lock()
		value, err := getter() // getter 획득동안 lock 유지
		cache.value = value
		cache.err = err
		cache.time = time.Now()
		cache.mutex.Unlock()
		return value, err
	} else {
		cache := &cacheResult{
			value: nil,
			err:   nil,
			time:  time.Now(),
			mutex: &sync.Mutex{},
		}

		cache.mutex.Lock()
		c.cache[key] = cache
		value, err := getter() // getter 획득동안 lock 유지
		cache.value = value
		cache.err = err
		cache.time = time.Now()
		cache.mutex.Unlock()
		return value, err
	}

}
