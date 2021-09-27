package docker

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pubg/kube-image-deployer/util"
)

type RemoteRegistryDocker struct {
	imageAuthMap    map[string]authn.Keychain
	defaultPlatform *v1.Platform
	cache           *util.Cache
}

// NewRemoteRegistry returns a new RemoteRegistryDocker
func NewRemoteRegistry() *RemoteRegistryDocker {
	d := &RemoteRegistryDocker{
		imageAuthMap:    make(map[string]authn.Keychain),
		cache:           util.NewCache(60),
		defaultPlatform: &v1.Platform{OS: "linux", Architecture: "amd64"},
	}

	return d
}

func (d *RemoteRegistryDocker) WithImageAuthMap(imageAuthMap map[string]authn.Keychain) *RemoteRegistryDocker {
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

// GetImage returns a docker image digest hash from url:tag
func (d *RemoteRegistryDocker) GetImageString(url, tag, platformString string) (string, error) {

	if strings.Contains(tag, "*") {
		// *을 포함하는 경우 전체 tag에서 가장 높은 tag를 찾아 반환한다.
		return d.getImageHighestVersionTag(url, tag, platformString)
	} else {
		// 단일 tag인 경우 가장 최신 sha256 digest를 반환한다. digest의 경우 platform이 필요하다.
		return d.getImageDigestHash(url, tag, platformString)
	}
}

func (d *RemoteRegistryDocker) getAuthKeyChain(url string) authn.Keychain {

	for key, value := range d.imageAuthMap {
		if strings.HasPrefix(url, key) {
			return value
		}
	}

	return authn.DefaultKeychain

}

func (d *RemoteRegistryDocker) getImageDigestHash(url, tag, platformString string) (string, error) {

	platform, err := d.parsePlatform(platformString)

	if err != nil {
		return "", err
	}

	fullUrl := fmt.Sprintf("%s:%s", url, tag)
	authKeyChain := d.getAuthKeyChain(url)
	ref, err := name.ParseReference(fullUrl)

	if err != nil {
		return "", err
	}

	hash, err := d.cache.Get(fullUrl, func() (interface{}, error) {
		if img, err := remote.Image(ref, remote.WithAuthFromKeychain(authKeyChain), remote.WithPlatform(*platform)); err == nil {
			if digest, err := img.Digest(); err == nil {
				return digest.String(), nil
			} else {
				return "", err
			}
		} else {
			return "", err
		}
	})

	return url + "@" + hash.(string), err

}

func (d *RemoteRegistryDocker) getImageHighestVersionTag(url, tag, platformString string) (string, error) {
	authKeyChain := d.getAuthKeyChain(url)
	repo, err := name.NewRepository(url)
	if nil != err {
		return "", err
	}

	cacheKey := url + "___" + tag
	image, err := d.cache.Get(cacheKey, func() (interface{}, error) {
		tags, err := remote.List(repo, remote.WithAuthFromKeychain(authKeyChain))
		if nil != err {
			return "", err
		}

		t, err := util.GetHighestVersionWithFilter(tags, tag)
		if nil != err {
			return "", err
		}

		return d.getImageDigestHash(url, t, platformString)
	})

	if nil != err {
		return "", err
	}

	return image.(string), nil
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
