// kube-image-deployer-cli 주어진 image:tag의 digest hash를 조회하는 CLI.
// 루트 모듈(github.com/pubg/kube-image-deployer)의 remoteRegistry를 직접 참조한다.
package main

import (
	"flag"
	"fmt"
	"os"

	docker "github.com/pubg/kube-image-deployer/remoteRegistry/docker"
)

var (
	image          string
	tag            string
	platformString string
)

func init() {
	flag.StringVar(&image, "image", "", "image url")
	flag.StringVar(&tag, "tag", "", "tag")
	flag.StringVar(&platformString, "platform", "linux/amd64", "platform string")
	flag.Parse()
}

func main() {

	if image == "" || tag == "" {
		flag.Usage()
		os.Exit(2)
	}

	registry := docker.NewRemoteRegistry()
	resolved, err := registry.GetImageString(image, tag, platformString)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s\n", resolved.ImageString)

}
