package docker

import (
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
	r, err := godotenv.Read()
	if err != nil {
		panic(err)
	}

	privateenv.host = r["TEST_DOCKER_PRIVATE_HOST"]
	privateenv.image = r["TEST_DOCKER_PRIVATE_IMAGE"]
	privateenv.tag = r["TEST_DOCKER_PRIVATE_TAG"]
	privateenv.auth = r["TEST_DOCKER_PRIVATE_AUTH"]
	privateenv.username = r["TEST_DOCKER_PRIVATE_USERNAME"]
	privateenv.password = r["TEST_DOCKER_PRIVATE_PASSWORD"]

	ecrenv.host = r["TEST_DOCKER_ECR_HOST"]
	ecrenv.image = r["TEST_DOCKER_ECR_IMAGE"]
	ecrenv.tag = r["TEST_DOCKER_ECR_TAG"]
}

func TestGetImageStringAsterisk(t *testing.T) {
	r := NewRemoteRegistry()

	if s, err := r.GetImageString("busybox", "1.34.*", "linux/amd64"); err != nil {
		t.Fatalf("TestGetImageStringAsterisk asterisk err: %v", err)
	} else if !strings.Contains(s, "@sha256:") {
		t.Fatalf("TestGetImageStringAsterisk asterisk no digest: %s", s)
	} else {
		t.Logf("TestGetImageStringAsterisk asterisk success: %s", s)
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
		t.Fatalf("TestGetImageFromPrivateRegistry env not set")
	}

	if s, err := r.GetImageString(privateenv.host+"/"+privateenv.image, privateenv.tag, "linux/amd64"); err != nil {
		t.Fatalf("TestGetImageFromPrivateRegistry err: %v", err)
	} else {
		t.Logf("TestGetImageFromPrivateRegistry success: %s", s)
	}
}

func TestGetImageFromECR(t *testing.T) {
	r := NewRemoteRegistry()

	if ecrenv.host == "" || ecrenv.image == "" || ecrenv.tag == "" {
		t.Fatalf("TestGetImageFromECR env not set")
	}

	if s, err := r.GetImageString(ecrenv.host+"/"+ecrenv.image, ecrenv.tag, "linux/amd64"); err != nil {
		t.Fatalf("TestGetImageFromECR err: %v, %+v", err, ecrenv)
	} else {
		t.Logf("TestGetImageFromECR success: %s", s)
	}
}
