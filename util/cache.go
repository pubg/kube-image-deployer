package util

import (
	"sync"
	"sync/atomic"
	"time"
)

type Cache struct {
	TTL uint

	cache                  map[string]*cacheResult
	mutex                  *sync.Mutex
	cacheGetterCalledCount uint32
}

type cacheResult struct {
	value interface{}
	err   error
	time  time.Time
	mutex *sync.Mutex
}

func NewCache(ttlSeconds uint) *Cache {
	return &Cache{
		TTL:                    ttlSeconds,
		cache:                  make(map[string]*cacheResult),
		mutex:                  &sync.Mutex{},
		cacheGetterCalledCount: 0,
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

	atomic.AddUint32(&c.cacheGetterCalledCount, 1)

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

		cache.mutex.Lock() // cache key mutex에 lock부터 걸고

		c.mutex.Lock() // cache map에 추가
		c.cache[key] = cache
		c.mutex.Unlock()

		value, err := getter() // getter 획득
		cache.value = value
		cache.err = err
		cache.time = time.Now()
		cache.mutex.Unlock() // unlock

		return value, err
	}

}
