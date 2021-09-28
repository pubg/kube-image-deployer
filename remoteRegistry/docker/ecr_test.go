package docker

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

type testEcrEnv struct {
	host  string
	image string
	tag   string
}

var ecrenv = testEcrEnv{}

func init() {
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Printf("error loading .env file - %s", err)
	}

	ecrenv.host = os.Getenv("TEST_DOCKER_ECR_HOST")
	ecrenv.image = os.Getenv("TEST_DOCKER_ECR_IMAGE")
	ecrenv.tag = os.Getenv("TEST_DOCKER_ECR_TAG")
}

func TestGetImageFromECR(t *testing.T) {

	if os.Getenv("TEST_DOCKER_ECR_SKIP") != "" {
		t.Log("skipping test")
		return
	}

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
