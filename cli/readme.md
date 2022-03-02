# kube-image-deployer-cli
> A command line interface for kube-image-deployer

# Build executables
> `go build -o bin/kube-image-deployer-cli ./src`

# Run the CLI
> `bin/kube-image-deployer-cli --image="busybox" --tag="latest"`
> busybox@sha256:b69959407d21e8a062e0416bf13405bb2b71ed7a84dde4158ebafacfa06f5578
>
> `bin/kube-image-deployer-cli --image="busybox" --tag="1.35.*"`
> busybox@sha256:e2a789e2f7d5faf46c80b772f19b469cbc2abe40e1c09258ca70299444d0d7cd

# Help
```bash
bin/kube-image-deployer-cli --help
Usage of bin/kube-image-deployer-cli:
  -image string
        image url
  -platform string
        platform string (default "linux/amd64")
  -tag string
        tag
```