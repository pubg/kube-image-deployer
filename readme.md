# kube-image-deployer

kube-image-deployer is a Kubernetes controller that closely monitors Docker Registry Image:Tag.

Unlike Keel, kube-image-deployer only tracks a single tag and operates in a more straightforward manner.

It observes both *Container* and *InitContainer* in supported Kubernetes Workloads such as *deployment*, *statefulset*, *daemonset*, and *cronjob*.

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

# How it Works
* Labels workloads with "kube-image-deployer" for monitoring.
* Uses annotations to keep track of which images and containers need to be monitored.
* Checks for updated image info and image digest hash from Docker registry API v2 every minute (imageStringCacheTTLSec).
* Makes changes to the workload's containers using the image digest hash, avoiding redeployment if only the tag changes but the image hash stays the same.

# Kubernetes Yaml Examples
## Essential YAML Configuration
* metadata.label.kube-image-deployer
  * This label is necessary to identify the workloads being monitored.
* metadata.annotations.kube-image-deployer/\${containerName} = \${ImageURL}:\${Tag}
  * This annotation records the container name, image URL, and tag for automatic updates.

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
For more information, refer to [kube-image-deployer-cli](cli)

# Installing in Kubernetes
Refer to the [Installation Guide in Kubernetes](docs/install-in-kubernetes.md) for further details.

# Using with Pulumi
For more information, refer to [docs/use-with-pulumi.md](docs/use-with-pulumi.md)

# Private Repositories
kube-image-deployer acquires the necessary access rights through Docker Credentials.

## Monitoring of Images in a Private Registry on DockerHub/Harbor
1. To access the private registry on Kubernetes, create a Dockerconfig secret.
1. Input the URL and authentication information, such as username and password, in the Auths section.
1. Mount the secret volume at the location of ```/root/.docker/config.json```.
1. The kube-image-deployer accesses the private registry by utilizing the mounted credentials in the Creds section, enabled by the AuthKeyChain.

## Monitoring of Images in a Private Registry on ECR
There are two methods available:
* Assign a role with ECR access permissions to the kube-image-deployer Service Account through AWS IRSA.
([#1](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html), [#2](https://docs.aws.amazon.com/ko_kr/AmazonECR/latest/userguide/ECR_on_EKS.html), [#3](https://aws.amazon.com/ko/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/)).
* Specify the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY with ECR access in the environment variables of the kube-image-deployer.

The kube-image-deployer automatically checks the ECR image URL by calling GetAuthorizationToken to get the Docker authentication token. With this token, it then retrieves the information about the image through the Docker Registry API v2.


# Todo
* Add Test Code
