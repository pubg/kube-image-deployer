package util

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// Cache에 같은 Key로 거의 동시에 get이 호출되는 경우 getter는 1회만 실행되어야 한다.
func TestCacheRaceKey(t *testing.T) {

	cache := NewCache(60)
	cnt := int32(0)

	f := func() chan bool {
		stopCh := make(chan bool)

		go func() {
			r, err := cache.Get("aaa", func() (interface{}, error) {
				<-time.After(time.Second * 1)
				fmt.Println("test")
				return "bbb", nil
			})

			if r == "bbb" && err == nil {
				atomic.AddInt32(&cnt, 1)
			}

			stopCh <- true
		}()
		return stopCh
	}

	stopCh1 := f()
	stopCh2 := f()

	<-stopCh1
	<-stopCh2

	if cnt != 2 || cache.cacheGetterCalledCount != 1 {
		t.Fatalf("TestCacheRaceKey getterCalledCount: %d", cache.cacheGetterCalledCount)
	}

}
