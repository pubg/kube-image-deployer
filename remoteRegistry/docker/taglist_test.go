package docker

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// newCountingRegistry tags/list 요청 횟수를 세는 in-memory registry를 띄운다.
func newCountingRegistry(t *testing.T, tagListCalls *int32) *httptest.Server {
	t.Helper()

	reg := registry.New(registry.Logger(log.New(io.Discard, "", 0)))

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			atomic.AddInt32(tagListCalls, 1)
		}
		reg.ServeHTTP(w, r)
	}))
}

func pushRandomImage(t *testing.T, ref string) string {
	t.Helper()

	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := name.ParseReference(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(parsed, img); err != nil {
		t.Fatal(err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return digest.String()
}

// 같은 저장소를 여러 태그 패턴이 감시할 때, 전체 태그 목록 조회(tags/list)는
// 캐시 TTL 내에서 저장소당 1번만 나가야 한다. 프로덕션에서는 저장소당 태그가
// 수천 개(페이지네이션 수 회)라 패턴마다 목록을 다시 받는 비용이 크다.
func TestTagListFetchedOncePerRepoAcrossPatterns(t *testing.T) {

	var tagListCalls int32
	server := newCountingRegistry(t, &tagListCalls)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	url := host + "/test/app"

	pushRandomImage(t, url+":1.0.1")
	digest1 := pushRandomImage(t, url+":1.0.2")
	digest2 := pushRandomImage(t, url+":2.0.1-dev")

	r := NewRemoteRegistry()

	// 같은 저장소에 서로 다른 두 패턴 (프로덕션의 *-pubg-dev / *-pubg-hotfix 병행 감시와 동일 구도)
	resolved1, err := r.GetImageString(url, "1.0.*", "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}
	resolved2, err := r.GetImageString(url, "2.0.*-dev", "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}

	// 해석 결과는 패턴별로 정확해야 한다
	if resolved1.Tag != "1.0.2" || resolved1.ImageString != url+"@"+digest1 {
		t.Fatalf("pattern 1.0.* resolved wrong: %+v", resolved1)
	}
	if resolved2.Tag != "2.0.1-dev" || resolved2.ImageString != url+"@"+digest2 {
		t.Fatalf("pattern 2.0.*-dev resolved wrong: %+v", resolved2)
	}

	if calls := atomic.LoadInt32(&tagListCalls); calls != 1 {
		t.Fatalf("tags/list must be fetched once per repo within TTL, got %d", calls)
	}

}

// 여러 패턴이 동시에 해석되어도(imageNotifier는 semaphore 8로 병렬 체크)
// 태그 목록 조회는 single-flight로 합쳐져 저장소당 1번이어야 한다.
func TestTagListSingleFlightUnderConcurrency(t *testing.T) {

	var tagListCalls int32
	server := newCountingRegistry(t, &tagListCalls)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	url := host + "/test/app"

	patterns := []string{"1.0.*", "2.0.*", "3.0.*"}
	for _, prefix := range []string{"1.0.1", "2.0.1", "3.0.1"} {
		pushRandomImage(t, url+":"+prefix)
	}

	r := NewRemoteRegistry()

	var wg sync.WaitGroup
	for _, pattern := range patterns {
		wg.Add(1)
		go func(pattern string) {
			defer wg.Done()
			if _, err := r.GetImageString(url, pattern, "linux/amd64"); err != nil {
				t.Errorf("GetImageString %s: %v", pattern, err)
			}
		}(pattern)
	}
	wg.Wait()

	if calls := atomic.LoadInt32(&tagListCalls); calls != 1 {
		t.Fatalf("concurrent patterns must share one tags/list fetch, got %d", calls)
	}

}
