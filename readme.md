# kube-image-deployer

kube-image-deployer는 Docker Registry의 Image:Tag를 감시하는 Kubernetes Controller입니다.

Keel과 유사하지만 단일 태그만 감시하며 더 간결하게 동작합니다.

Container, InitContainer를 모두 감시합니다.

현재 지원되는 Workload는 다음과 같습니다.
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

# 동작 방식
* kube-image-deployer label을 가진 Workload를 감시 대상으로 등록 합니다.
* Workload의 annotation을 읽어 감시할 Image와 Container를 매핑합니다.
* 1분 간격(imageStringCacheTTLSec)으로 Docker Registry API v2로 이미지 정보와 이미지의 Digest Hash를 획득해 해당 사용중인 Workload의 Container에 Strategic Merge Patch를 진행합니다.
* Image Digest Hash로 패치하기 때문에 새 태그만 추가되고 이미지 Hash가 변경되지 않은 경우는 Workload가 재배포 되지 않습니다. (의도됨)

# Kubernetes Yaml Examples
## Yaml 필수 구성 요소
* metadata.label.kube-image-deployer
  * label을 가진 Workload를 감시하게 되므로 필수입니다.
* metadata.annotations.kube-image-deployer/\${containerName} = \${ImageURL}:\${Tag}
  * 자동 업데이트를 동작시킬 컨테이너 이름과 이미지, 태그를 Annotation에 등록합니다.

## Tag 감시 방식
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

# Private Repositories
kube-image-deployer는 Docker Creds로 기본 접근 권한을 획득합니다.

## DockerHub / Harbor의 Private Registry 이미지 감시하기
1. Kubernetes에 Private Registry 접근용 dockerconfig Secret을 생성합니다.
1. Auths에 URL과 접근 방법(username/password...) 입력합니다.
1. ```/root/.docker/config.json``` 위치에 Secret Volume을 마운트합니다.
1. kube-image-deployer는 AuthKeyChain을 통해 Creds에 마운트된 정보로 Private Registry를 접근합니다.

## ECR의 Private Registry 이미지 감시하기
두 가지 방법이 있습니다.
1. kube-image-deployer Service Account에 ECR 접근 권한을 가진 Role을 설정해 권한을 주는 방법 (AWS IRSA)
1. kube-image-deployer env에 ECR 접근 가능한 AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY를 입력하는 방법 (AWS AccessToken)

ECR 이미지 URL이 감지대상인 경우 kube-image-deloyer는 ECR의 GetAuthorizationToken을 호출해 Docker Auth Token을 획득하고 이 토큰을 사용해 Docker Registry API v2로 이미지 정보를 획득합니다.

# Todo
* Add Test Code
