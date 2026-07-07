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
	value any
	valid bool // getter가 성공해 value가 유효한지 여부
	time  time.Time
	mutex sync.Mutex
}

func NewCache(ttlSeconds uint) *Cache {
	return &Cache{
		TTL:                    ttlSeconds,
		cache:                  make(map[string]*cacheResult),
		mutex:                  &sync.Mutex{},
		cacheGetterCalledCount: 0,
	}
}

// Get key에 대한 캐시된 값을 반환한다. 만료되었거나 없으면 getter를 호출해 갱신한다.
// 같은 key에 대한 getter 호출은 직렬화된다(single-flight).
// getter가 실패한 경우 에러는 캐시하지 않으며 다음 호출에서 재시도한다.
func (c *Cache) Get(key string, getter func() (any, error)) (any, error) {

	c.mutex.Lock()
	entry, ok := c.cache[key]
	if !ok {
		entry = &cacheResult{}
		c.cache[key] = entry
	}
	c.mutex.Unlock()

	entry.mutex.Lock()
	defer entry.mutex.Unlock()

	// 잠금 대기 중 다른 goroutine이 이미 갱신했을 수 있으므로 잠금 획득 후 재확인한다
	if entry.valid && time.Since(entry.time) < time.Duration(c.TTL)*time.Second {
		return entry.value, nil
	}

	atomic.AddUint32(&c.cacheGetterCalledCount, 1)

	value, err := getter()

	if err != nil {
		return nil, err // 에러는 캐시하지 않는다
	}

	entry.value = value
	entry.valid = true
	entry.time = time.Now()

	return value, nil

}
