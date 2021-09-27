package docker

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
)

type PrivateAuthenticator struct {
	url string

	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}

func (e *PrivateAuthenticator) Authorization() (*authn.AuthConfig, error) {

	if e.Username != "" && e.Password != "" {
		return &authn.AuthConfig{
			Username: e.Username,
			Password: e.Password,
		}, nil
	} else if e.IdentityToken != "" {
		return &authn.AuthConfig{
			IdentityToken: e.IdentityToken,
		}, nil
	} else if e.RegistryToken != "" {
		return &authn.AuthConfig{
			RegistryToken: e.RegistryToken,
		}, nil
	} else if e.Auth != "" {
		return &authn.AuthConfig{
			Auth: e.Auth,
		}, nil
	}

	return nil, fmt.Errorf("no auth config found %+v", e)
}

func NewPrivateAuthenticator(url, username, password string) *PrivateAuthenticator {
	return &PrivateAuthenticator{
		url:      url,
		Username: username,
		Password: password,
	}
}

func NewPrivateAuthenticatorWithIdentityToken(url, identityToken string) *PrivateAuthenticator {
	return &PrivateAuthenticator{
		url:           url,
		IdentityToken: identityToken,
	}
}

func NewPrivateAuthenticatorWithRegistryToken(url, registryToken string) *PrivateAuthenticator {
	return &PrivateAuthenticator{
		url:           url,
		RegistryToken: registryToken,
	}
}

func NewPrivateAuthenticatorWithAuth(url, auth string) *PrivateAuthenticator {
	return &PrivateAuthenticator{
		url:  url,
		Auth: auth,
	}
}
