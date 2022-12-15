# kube-image-deployer

kube-image-deployer is a Kubernetes controller that monitors Docker Registry Image:Tag.

It is similar to Keel, but only monitors a single tag and operates more concisely.

It monitors both Container and InitContainer. Currently supported Workloads are:
 * deployment
 * statefulset
 * daemonset
 * cronjob


# Available Environment Flags
```go
kubeconfig               = *flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
offDeployments           = *flag.Bool("off-deployments", false, "disable deployments")
offStatefulsets          = *flag.Bool("off-statefulsets", false, "disable statefulsets")
offDaemonsets            = *flag.Bool("off-daemonsets", false, "disable daemonsets")
offCronjobs              = *flag.Bool("off-cronjobs", false, "disable cronjobs")
useCronJobV1             = *flag.Bool("use-cronjob-v1", false, "use cronjob version v1 instead of v1beta1")
imageStringCacheTTLSec   = *flag.Uint("image-hash-cache-ttl-sec", 60, "image hash cache TTL in seconds")
imageCheckIntervalSec    = *flag.Uint("image-check-interval-sec", 10, "image check interval in seconds")
controllerWatchKey       = *flag.String("controller-watch-key", "kube-image-deployer", "controller watch key")
controllerWatchNamespace = *flag.String("controller-watch-namespace", "", "controller watch namespace. If empty, watch all namespaces")
imageDefaultPlatform     = *flag.String("image-default-platform", "linux/amd64", "default platform for docker images")
slackWebhook             = *flag.String("slack-webhook", "", "slack webhook url. If empty, notifications are disabled")
slackMsgPrefix           = *flag.String("slack-msg-prefix", "[$hostname]", "slack message prefix. default=[hostname]")
```

# Available Environment Variables
```shell
KUBECONFIG_PATH=<absolute path to the kubeconfig file>
OFF_DEPLOYMENTS=<true>
OFF_STATEFULSETS=<true>
OFF_DAEMONSETS=<true>
OFF_CRONJOBS=<true>
USE_CRONJOB_V1=<true>
IMAGE_HASH_CACHE_TTL_SEC=<uint>
IMAGE_CHECK_INTERVAL_SEC=<uint>
CONTROLLER_WATCH_KEY=<kube-image-deployer>
CONTROLLER_WATCH_NAMESPACE=<controller watch namespace. If empty, watch all namespaces>
IMAGE_DEFAULT_PLATFORM=<default platform for docker images>
SLACK_WEBHOOK=<slack webhook url. If empty, notifications are disabled>
SLACK_MSG_PREFIX=<slack message prefix. default=[hostname]>
```

# How it works
* Registers workloads with the "kube-image-deployer" label as targets for monitoring.
* Reads the workload's annotations to map the images and containers to be monitored.
* Acquires image information and image Digest Hash from Docker Registry API v2 at a 1-minute interval (imageStringCacheTTLSec), and performs a Strategic Merge Patch on the workload's containers.
* Because the patch is performed using the Image Digest Hash, the workload will not be redeployed if only the new tag is added and the image hash remains unchanged (intended).

# Kubernetes Yaml Examples
## Required Yaml Configuration
* metadata.label.kube-image-deployer
  * This is required because it labels the workloads that are monitored.
* metadata.annotations.kube-image-deployer/\${containerName} = \${ImageURL}:\${Tag}
  * Registers the container name, image, and tag for automatic updates in the annotations.

## Tag Monitoring Method
* Exact match tag
  * busybox:1.34.0 -> busybox@sha256:15f840677a5e245d9ea199eb9b026b1539208a5183621dced7b469f6aa678115

* Asterisk match tag
  * busybox:1.34.* -> 1.34.0, 1.34.1, 1.34.2, ... -> busybox@sha256:15f840677a5e245d9ea199eb9b026b1539208a5183621dced7b469f6aa678115

## Yaml Samples
### Deployments
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    kube-image-deployer/busybox-init: 'busybox:latest' # set init container update
    kube-image-deployer/busybox2: 'busybox:1.34.*'     # set container update
  labels:
    app: kube-image-deployer-test
    kube-image-deployer: 'true'                        # enable kube-image-deployer
  name: kube-image-deployer-test
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kube-image-deployer-test
  template:
    metadata:
      labels:
        app: kube-image-deployer-test
    spec:
      containers:
        - name: busybox
          image: busybox # no change
          args: ['sleep', '1000000']
        - name: busybox2
          image: busybox # change to busybox@sha:b862520da7361ea093806d292ce355188ae83f21e8e3b2a3ce4dbdba0a230f83
          args: ['sleep', '1000000']
      initContainers:
        - name: busybox-init
          image: busybox # change to busybox@sha:b862520da7361ea093806d292ce355188ae83f21e8e3b2a3ce4dbdba0a230f83
```
# Using kube-image-deployer as [CLI](cli)
See : [kube-image-deployer-cli](cli)

# Install in Kubernetes
See : [docs/install-in-kubernetes.md](docs/install-in-kubernetes.md)

# Use with Pulumi
See : [docs/use-with-pulumi.md](docs/use-with-pulumi.md)

# Private Repositories
kube-image-deployer obtains basic access permission through Docker Creds.

## Monitoring Private Registry Images on DockerHub / Harbor
1. Create a dockerconfig Secret for accessing the Private Registry on Kubernetes.
1. Input the URL and access method (username/password...) in Auths.
1. Mount the Secret Volume in the ```/root/.docker/config.json``` location.
1. kube-image-deployer accesses the Private Registry through the information mounted in Creds using the AuthKeyChain.

## Monitoring Private Registry Images on ECR
There are two methods:
1. Setting a Role with ECR access permission on the kube-image-deployer Service Account (AWS IRSA)
1. Inputting AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY that can access ECR in the kube-image-deployer env (AWS AccessToken)

If the ECR image URL is the target of detection, kube-image-deloyer calls ECR's GetAuthorizationToken to obtain the Docker Auth Token, and uses this token to acquire image information through the Docker Registry API v2.


# Todo
* Add Test Code
