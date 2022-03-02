package main

import (
	"flag"
	"fmt"

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
		return
	}

	registry := docker.NewRemoteRegistry()
	s, err := registry.GetImageString(image, tag, "linux/amd64")

	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", s)

}
