package docker

import (
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pubg/kube-image-deployer/util"
	"k8s.io/klog/v2"
)

type ECRAuthenticator struct {
	url    string
	region string
}

func getRegionFromECRURL(url string) string {
	return isECRRegex.FindStringSubmatch(url)[1]
}

func (e *ECRAuthenticator) Authorization() (*authn.AuthConfig, error) {

	token, err := ecrCache.Get(e.url, func() (interface{}, error) {

		sess := session.Must(session.NewSessionWithOptions(session.Options{}))
		svc := ecr.New(sess, aws.NewConfig().WithRegion(e.region))
		if token, err := svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{}); err != nil {
			klog.Errorf("ECRAuthenticator GetAuthorizationToken error url=%s, err=%v", e.url, err)
			return nil, err
		} else {
			klog.V(4).Infof("ECRAuthenticator GetAuthorizationToken success url=%s, token=%v", e.url, token)
			return *token.AuthorizationData[0].AuthorizationToken, nil
		}
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

func NewECRAuthenticator(url string) *ECRAuthenticator {
	return &ECRAuthenticator{
		url:    url,
		region: getRegionFromECRURL(url),
	}
}
