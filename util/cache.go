package util

import (
	"sync"
	"time"
)

type Cache struct {
	TTL uint

	cache map[string]cacheResult
	mutex sync.Mutex
}

type cacheResult struct {
	value interface{}
	err   error
	time  time.Time
}

func NewCache(ttlSeconds uint) *Cache {
	return &Cache{
		TTL:   ttlSeconds,
		cache: make(map[string]cacheResult),
	}
}

func (c *Cache) Get(key string, getter func() (interface{}, error)) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if res, ok := c.cache[key]; ok && time.Since(res.time) < time.Duration(c.TTL)*time.Second {
		return res.value, res.err
	}

	value, err := getter()
	res := cacheResult{
		value: value,
		err:   err,
		time:  time.Now(),
	}

	c.cache[key] = res
	
	return res.value, res.err

}
