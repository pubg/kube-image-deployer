package docker

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/joho/godotenv"
)

type testPrivateEnv struct {
	host     string
	image    string
	tag      string
	auth     string
	username string
	password string
}

var privateenv = testPrivateEnv{}

func init() {
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Printf("error loading .env file - %s", err)
	}

	privateenv.host = os.Getenv("TEST_DOCKER_PRIVATE_HOST")
	privateenv.image = os.Getenv("TEST_DOCKER_PRIVATE_IMAGE")
	privateenv.tag = os.Getenv("TEST_DOCKER_PRIVATE_TAG")
	privateenv.auth = os.Getenv("TEST_DOCKER_PRIVATE_AUTH")
	privateenv.username = os.Getenv("TEST_DOCKER_PRIVATE_USERNAME")
	privateenv.password = os.Getenv("TEST_DOCKER_PRIVATE_PASSWORD")
}

func TestGetImageFromPrivateRegistry(t *testing.T) {
	if os.Getenv("TEST_DOCKER_PRIVATE_SKIP") != "" {
		t.Log("skipping test")
		return
	}

	r := NewRemoteRegistry()

	if privateenv.auth != "" {
		r.WithImageAuthMap(map[string]authn.Authenticator{
			privateenv.host: NewPrivateAuthenticatorWithAuth(privateenv.host, privateenv.auth),
		})
	} else if privateenv.username != "" && privateenv.password != "" {
		r.WithImageAuthMap(map[string]authn.Authenticator{
			privateenv.host: NewPrivateAuthenticator(privateenv.host, privateenv.username, privateenv.password),
		})
	} else {
		t.Fatalf("env not set")
	}

	if s, err := r.GetImageString(privateenv.host+"/"+privateenv.image, privateenv.tag, "linux/amd64"); err != nil {
		t.Fatalf("err: %v", err)
	} else {
		t.Logf("success: %s", s)
	}
}
