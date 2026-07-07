package docker

import (
	"testing"

	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/util"
)

// ECR 토큰은 레지스트리(계정+리전) 단위로 유효하므로, 같은 레지스트리의 저장소가
// 여러 개여도 GetAuthorizationToken은 1번만 호출되어야 한다.
// (프로덕션에서 같은 계정의 저장소 28개가 각각 토큰을 획득하던 문제의 재현)
func TestECRTokenSharedAcrossReposOfSameRegistry(t *testing.T) {

	origFetch, origCache := fetchECRAuthorizationToken, ecrCache
	defer func() { fetchECRAuthorizationToken, ecrCache = origFetch, origCache }()

	calls := 0
	fetchECRAuthorizationToken = func(region string) (string, error) {
		calls++
		return "token-" + region, nil
	}
	ecrCache = util.NewCache(3600)

	log := logger.NewLogger()

	// 같은 레지스트리의 서로 다른 저장소 3개
	for _, repo := range []string{"pubg_anticheat/ac-bb-server", "pubg_anticheat/zk-admin-server", "pubg_anticheat/yperagathou"} {
		auth := NewECRAuthenticator("480544169436.dkr.ecr.us-east-1.amazonaws.com/"+repo, log)
		c, err := auth.Authorization()
		if err != nil {
			t.Fatal(err)
		}
		if c.Auth != "token-us-east-1" {
			t.Fatalf("unexpected token: %+v", c)
		}
	}

	if calls != 1 {
		t.Fatalf("same registry must share one token fetch, got %d", calls)
	}

	// 다른 레지스트리(다른 리전)는 별도로 토큰을 획득해야 한다
	auth := NewECRAuthenticator("480544169436.dkr.ecr.ap-northeast-2.amazonaws.com/other/repo", log)
	if c, err := auth.Authorization(); err != nil || c.Auth != "token-ap-northeast-2" {
		t.Fatalf("different registry must fetch its own token: %+v, %v", c, err)
	}
	if calls != 2 {
		t.Fatalf("different registry must fetch separately, got %d", calls)
	}

}
