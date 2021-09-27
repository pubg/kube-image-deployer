package docker

import (
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/joho/godotenv"
)

type testenv struct {
	host     string
	image    string
	tag      string
	auth     string
	username string
	password string
}

var privateenv = testenv{}
var ecrenv = testenv{}

func init() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	privateenv.host = os.Getenv("TEST_DOCKER_PRIVATE_HOST")
	privateenv.image = os.Getenv("TEST_DOCKER_PRIVATE_IMAGE")
	privateenv.tag = os.Getenv("TEST_DOCKER_PRIVATE_TAG")
	privateenv.auth = os.Getenv("TEST_DOCKER_PRIVATE_AUTH")
	privateenv.username = os.Getenv("TEST_DOCKER_PRIVATE_USERNAME")
	privateenv.password = os.Getenv("TEST_DOCKER_PRIVATE_PASSWORD")

	ecrenv.host = os.Getenv("TEST_DOCKER_ECR_HOST")
	ecrenv.image = os.Getenv("TEST_DOCKER_ECR_IMAGE")
	ecrenv.tag = os.Getenv("TEST_DOCKER_ECR_TAG")
}

func TestGetImageStringAsterisk(t *testing.T) {
	r := NewRemoteRegistry()

	if s, err := r.GetImageString("busybox", "1.34.*", "linux/amd64"); err != nil {
		t.Fatalf("err: %v", err)
	} else if !strings.HasPrefix(s, "busybox@sha256:") {
		t.Fatalf("no digest: %s", s)
	} else {
		t.Logf("success: %s", s)
	}
}

func TestGetImageFromPrivateRegistry(t *testing.T) {
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

func TestGetImageFromECR(t *testing.T) {
	r := NewRemoteRegistry()

	if ecrenv.host == "" || ecrenv.image == "" || ecrenv.tag == "" {
		t.Fatalf("env not set")
	}

	if s, err := r.GetImageString(ecrenv.host+"/"+ecrenv.image, ecrenv.tag, "linux/amd64"); err != nil {
		t.Fatalf("err: %v, %+v", err, ecrenv)
	} else {
		t.Logf("success: %s", s)
	}
}
