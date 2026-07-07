package docker

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pubg/kube-image-deployer/interfaces"
	"github.com/pubg/kube-image-deployer/util"
)

type ECRAuthenticator struct {
	url    string
	host   string // 레지스트리 호스트 (aws_account_id.dkr.ecr.region.amazonaws.com)
	region string
	logger interfaces.ILogger
}

func NewECRAuthenticator(url string, logger interfaces.ILogger) *ECRAuthenticator {
	return &ECRAuthenticator{
		url:    url,
		host:   strings.SplitN(url, "/", 2)[0],
		region: getRegionFromECRURL(url),
		logger: logger,
	}
}

// fetchECRAuthorizationToken 실제 ECR GetAuthorizationToken 호출. 테스트에서 대체한다.
var fetchECRAuthorizationToken = func(region string) (string, error) {

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", err
	}

	svc := ecr.NewFromConfig(cfg)
	token, err := svc.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})

	if err != nil {
		return "", err
	}

	if len(token.AuthorizationData) == 0 || token.AuthorizationData[0].AuthorizationToken == nil {
		return "", fmt.Errorf("empty AuthorizationData")
	}

	return *token.AuthorizationData[0].AuthorizationToken, nil
}

func (e *ECRAuthenticator) Authorization() (*authn.AuthConfig, error) {

	// 토큰은 레지스트리(계정+리전) 단위로 유효하므로 저장소가 달라도 host 단위로 공유한다
	token, err := ecrCache.Get(e.host, func() (any, error) {

		token, err := fetchECRAuthorizationToken(e.region)

		if err != nil {
			e.logger.Errorf("ECRAuthenticator GetAuthorizationToken error host=%s, err=%v", e.host, err)
			return nil, err
		}

		// 토큰 갱신 성공은 루틴 동작이므로 klog에만 남긴다 (Slack 미러 제외)
		e.logger.Debugf("ECRAuthenticator GetAuthorizationToken success host=%s", e.host)
		return token, nil
	})

	if err != nil {
		return nil, err
	}

	c := &authn.AuthConfig{
		Auth: token.(string),
	}

	return c, nil
}

// aws_account_id.dkr.ecr.region.amazonaws.com
var isECRRegex = regexp.MustCompile(`\d+\.dkr\.ecr\.(.+)\.amazonaws.com`)
var ecrCache = util.NewCache(60 * 60 * 11) // 11 hours

// isECR returns true if the url is ECR repository.
// ex> aws_account_id.dkr.ecr.region.amazonaws.com
func isECR(url string) bool {
	return isECRRegex.Match([]byte(url))
}

func getRegionFromECRURL(url string) string {
	return isECRRegex.FindStringSubmatch(url)[1]
}
