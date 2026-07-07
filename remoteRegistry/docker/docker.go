package docker

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pubg/kube-image-deployer/interfaces"
	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/util"
)

type RemoteRegistryDocker struct {
	imageAuthMap    map[string]authn.Authenticator
	defaultPlatform *v1.Platform
	cache           *util.Cache
	logger          interfaces.ILogger
}

// NewRemoteRegistry returns a new RemoteRegistryDocker
func NewRemoteRegistry() *RemoteRegistryDocker {
	d := &RemoteRegistryDocker{
		imageAuthMap:    make(map[string]authn.Authenticator),
		cache:           util.NewCache(60),
		defaultPlatform: &v1.Platform{OS: "linux", Architecture: "amd64"},
		logger:          logger.NewLogger(),
	}

	return d
}

func (d *RemoteRegistryDocker) WithLogger(logger interfaces.ILogger) *RemoteRegistryDocker {
	d.logger = logger
	return d
}

func (d *RemoteRegistryDocker) WithImageAuthMap(imageAuthMap map[string]authn.Authenticator) *RemoteRegistryDocker {
	d.imageAuthMap = imageAuthMap
	return d
}

func (d *RemoteRegistryDocker) WithCache(cacheTTL uint) *RemoteRegistryDocker {
	d.cache = util.NewCache(cacheTTL)
	return d
}

func (d *RemoteRegistryDocker) WithDefaultPlatform(platformString string) *RemoteRegistryDocker {

	if platform, err := d.parsePlatform(platformString); err != nil {
		panic(err)
	} else {
		d.defaultPlatform = platform
	}
	return d
}

// GetImageString url:tag의 digest hash와 그 digest가 취득된 원본 태그를 반환한다.
func (d *RemoteRegistryDocker) GetImageString(url, tag, platformString string) (interfaces.ResolvedImage, error) {

	if strings.Contains(tag, "*") {
		// *을 포함하는 경우 전체 tag에서 가장 높은 tag를 찾아 반환한다.
		return d.getImageHighestVersionTag(url, tag, platformString)
	}

	// 단일 tag인 경우 가장 최신 sha256 digest를 반환한다. digest의 경우 platform이 필요하다.
	imageString, err := d.getImageDigestHash(url, tag, platformString)

	if err != nil {
		return interfaces.ResolvedImage{}, err
	}

	return interfaces.ResolvedImage{ImageString: imageString, Tag: tag}, nil
}

func (d *RemoteRegistryDocker) getAuthenticator(url string) authn.Authenticator {

	for key, value := range d.imageAuthMap {
		if strings.HasPrefix(url, key) {
			return value
		}
	}

	if isECR(url) { // image가 ecr private repository 인 경우
		return NewECRAuthenticator(url, d.logger)
	}

	return nil

}

func (d *RemoteRegistryDocker) getRemoteOptions(url string) []remote.Option {
	var options []remote.Option = []remote.Option{}

	if auth := d.getAuthenticator(url); auth != nil {
		options = append(options, remote.WithAuth(auth))
	} else {
		options = append(options, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	return options
}

func (d *RemoteRegistryDocker) getImageDigestHash(url, tag, platformString string) (string, error) {

	platform, err := d.parsePlatform(platformString)

	if err != nil {
		return "", err
	}

	fullUrl := fmt.Sprintf("%s:%s", url, tag)
	options := d.getRemoteOptions(url)
	options = append(options, remote.WithPlatform(*platform))
	ref, err := name.ParseReference(fullUrl)

	if err != nil {
		return "", err
	}

	// platform 별로 digest가 다르므로 캐시 키에 platform을 포함한다
	cacheKey := fullUrl + "?platform=" + platform.String()

	hash, err := d.cache.Get(cacheKey, func() (any, error) {
		if img, err := remote.Image(ref, options...); err == nil {
			if digest, err := img.Digest(); err == nil {
				return digest.String(), nil
			} else {
				return "", err
			}
		} else {
			return "", err
		}
	})

	if err != nil {
		return "", err
	}

	return url + "@" + hash.(string), nil

}

// getImageHighestVersionTag 와일드카드 태그에서 최고 버전 태그를 찾아 digest와 함께 반환한다.
func (d *RemoteRegistryDocker) getImageHighestVersionTag(url, tag, platformString string) (interfaces.ResolvedImage, error) {
	options := d.getRemoteOptions(url)
	repo, err := name.NewRepository(url)
	if nil != err {
		return interfaces.ResolvedImage{}, err
	}

	platform, err := d.parsePlatform(platformString)

	if err != nil {
		return interfaces.ResolvedImage{}, err
	}

	cacheKey := url + "___" + tag + "?platform=" + platform.String()
	image, err := d.cache.Get(cacheKey, func() (any, error) {
		tags, err := remote.List(repo, options...)
		if nil != err {
			return nil, err
		}

		t, err := util.GetHighestVersionWithFilter(tags, tag)
		if nil != err {
			return nil, err
		}

		imageString, err := d.getImageDigestHash(url, t, platformString)
		if nil != err {
			return nil, err
		}

		// digest가 어느 태그에서 취득되었는지 함께 보존한다 (알림 표기에 사용)
		return interfaces.ResolvedImage{ImageString: imageString, Tag: t}, nil
	})

	if nil != err {
		return interfaces.ResolvedImage{}, err
	}

	return image.(interfaces.ResolvedImage), nil
}

func (d *RemoteRegistryDocker) parsePlatform(platformString string) (*v1.Platform, error) {

	if platformString == "" {
		return d.defaultPlatform, nil
	}

	arr := strings.Split(platformString, "/")
	if len(arr) != 2 {
		return d.defaultPlatform, fmt.Errorf("invalid platform string : %s", platformString)
	}
	return &v1.Platform{OS: arr[0], Architecture: arr[1]}, nil
}
