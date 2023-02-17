# kube-image-deployer-cli
The kube-image-deployer-cli is a command-line interface for kube-image-deployer that allows you to check the image digest hash of a given image and tag combination. Here's how to build and run the CLI:

1. Build the executable:
      ```bash
      go build -o bin/kube-image-deployer-cli ./src
      ```

1. Run the CLI with the --image and --tag flags to check the image digest hash for a specific image and tag combination. For example:
      ```bash
      bin/kube-image-deployer-cli --image="busybox" --tag="latest"
      busybox@sha256:b69959407d21e8a062e0416bf13405bb2b71ed7a84dde4158ebafacfa06f5578
      
      bin/kube-image-deployer-cli --image="busybox" --tag="1.35.*"
      busybox@sha256:e2a789e2f7d5faf46c80b772f19b469cbc2abe40e1c09258ca70299444d0d7cd
      ```

      The CLI returns the image digest hash in the format <image>@<hash>.

You can use the --platform flag to specify the platform of the image. The default value is linux/amd64.

You can also run bin/kube-image-deployer-cli --help to see the available flags and their usage.

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