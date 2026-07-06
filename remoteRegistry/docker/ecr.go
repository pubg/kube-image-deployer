package docker

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pubg/kube-image-deployer/interfaces"
	"github.com/pubg/kube-image-deployer/util"
)

type ECRAuthenticator struct {
	url    string
	region string
	logger interfaces.ILogger
}

func NewECRAuthenticator(url string, logger interfaces.ILogger) *ECRAuthenticator {
	return &ECRAuthenticator{
		url:    url,
		region: getRegionFromECRURL(url),
		logger: logger,
	}
}

func (e *ECRAuthenticator) Authorization() (*authn.AuthConfig, error) {

	token, err := ecrCache.Get(e.url, func() (interface{}, error) {

		ctx := context.Background()

		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(e.region))
		if err != nil {
			e.logger.Errorf("ECRAuthenticator LoadDefaultConfig error url=%s, err=%v", e.url, err)
			return nil, err
		}

		svc := ecr.NewFromConfig(cfg)
		token, err := svc.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})

		if err != nil {
			e.logger.Errorf("ECRAuthenticator GetAuthorizationToken error url=%s, err=%v", e.url, err)
			return nil, err
		}

		if len(token.AuthorizationData) == 0 || token.AuthorizationData[0].AuthorizationToken == nil {
			err := fmt.Errorf("empty AuthorizationData url=%s", e.url)
			e.logger.Errorf("ECRAuthenticator GetAuthorizationToken %v", err)
			return nil, err
		}

		e.logger.Infof("ECRAuthenticator GetAuthorizationToken success url=%s", e.url)
		return *token.AuthorizationData[0].AuthorizationToken, nil
	})

	if err != nil {
		return nil, err
	}

	c := &authn.AuthConfig{
		Auth: token.(string),
	}

	return c, nil
}

//aws_account_id.dkr.ecr.region.amazonaws.com
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
