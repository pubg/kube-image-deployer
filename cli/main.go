// kube-image-deployer-cli 주어진 image:tag의 digest hash를 조회하는 CLI.
// 루트 모듈(github.com/pubg/kube-image-deployer)의 remoteRegistry를 직접 참조한다.
//
// 출력 계약: 성공 시 stdout에 digest 한 줄만 출력하고 stderr는 침묵한다.
// 실패는 exit code(비0)와 stderr의 "error: ..." 메시지로 전달한다.
// (IaC 등 호출자가 stderr 출력 유무로 실패를 감지하는 경우가 있다)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	docker "github.com/pubg/kube-image-deployer/remoteRegistry/docker"
	"k8s.io/klog/v2"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// configureLogging 라이브러리(klog) 로그의 stderr 출력 정책을 설정한다.
// 기본은 억제: 정보성 klog가 stderr로 새면 호출자가 성공 결과를 실패로
// 오판할 수 있다. 실패 전달은 run()의 exit code와 error 메시지가 담당한다.
func configureLogging(verbose bool) {
	if verbose {
		return // klog 기본값(stderr 출력) 유지
	}
	klog.LogToStderr(false)
	klog.SetOutput(io.Discard)
}

func run(args []string, stdout, stderr io.Writer) int {

	fs := flag.NewFlagSet("kube-image-deployer-cli", flag.ExitOnError)
	fs.SetOutput(stderr)

	image := fs.String("image", "", "image url")
	tag := fs.String("tag", "", "tag")
	platformString := fs.String("platform", "linux/amd64", "platform string")
	verbose := fs.Bool("verbose", false, "print library logs (klog) to stderr for debugging")

	fs.Parse(args)

	configureLogging(*verbose)

	if *image == "" || *tag == "" {
		fs.Usage()
		return 2
	}

	registry := docker.NewRemoteRegistry()
	resolved, err := registry.GetImageString(*image, *tag, *platformString)

	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "%s\n", resolved.ImageString)
	return 0
}
