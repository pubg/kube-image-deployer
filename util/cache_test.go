package util

import (
	"errors"
	"fmt"
	"sync"
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
			r, err := cache.Get("aaa", func() (any, error) {
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

// getter가 실패하면 그 에러를 TTL 동안 캐시하지 말고 다음 호출에서 재시도해야 한다.
// (ECR 토큰 캐시는 TTL이 11시간이라, 일시 장애가 캐시되면 11시간 동안 인증이 전부 실패한다)
func TestCacheErrorNotCached(t *testing.T) {

	cache := NewCache(60)
	calls := 0

	if _, err := cache.Get("key", func() (any, error) {
		calls++
		return nil, errors.New("transient failure")
	}); err == nil {
		t.Fatal("expected error from first getter")
	}

	v, err := cache.Get("key", func() (any, error) {
		calls++
		return "recovered", nil
	})

	if err != nil || v != "recovered" {
		t.Fatalf("error must not be cached: v=%v err=%v", v, err)
	}

	if calls != 2 {
		t.Fatalf("getter should be retried after error, calls=%d", calls)
	}

	// 성공한 값은 캐시되어야 한다
	if v, err := cache.Get("key", func() (any, error) {
		calls++
		return "should-not-be-called", nil
	}); err != nil || v != "recovered" {
		t.Fatalf("success value must be cached: v=%v err=%v", v, err)
	}

	if calls != 2 {
		t.Fatalf("getter must not be called for cached value, calls=%d", calls)
	}

}

// 만료 갱신 경로(쓰기)와 TTL 체크 경로(읽기)가 동시에 실행되어도 데이터 레이스가 없어야 한다.
// go test -race에서 검증된다.
func TestCacheConcurrentReadWriteRace(t *testing.T) {

	cache := NewCache(0) // TTL 0: 항상 만료 → 갱신과 조회가 상시 경합

	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 200 {
				if _, err := cache.Get("key", func() (any, error) {
					return j, nil
				}); err != nil {
					t.Errorf("unexpected err: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

}
