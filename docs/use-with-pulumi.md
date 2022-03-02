# Use with Pulumi
In our case, we used [Pulumi](https://pulumi.com) as our Kubernetes IaC. It explains how to integrate with Pulumi, which was needed at this time.

### [Pulumi Transformation](https://www.pulumi.com/docs/intro/concepts/resources/options/transformations/) steps to prevent redistribution of past images when performing Pulumi Up.
> When ```pulumi up```, the image may be up-to-date due to CI/CD that has already occurred elsewhere. In order to prevent redistribution after returning to the past image due to pulumi up, you need to bring the currently deployed image hash when pulumi up.
1. Check whether annotation rule for kube-image-deployer is set in Pulumi Transformation.
    kube-image-deployer/[container.name]: '[container.image:tagExpr]'
    ``` 
    # If you want to deploy a container image like below
    containers:
        - name: main
          image: busybox:1.31.*
          ...
    # set annotation like below
    annotations:
        kube-image-deployer/main: 'busybox:1.31.*'
    ```
1. Parse the annotation to get the latest hash of the image. And set the Hash on the container image.
    ```
    containers:
        - name: main
          image: "busybox@sha256:fd4a8673d0344c3a7f427fe4440d4b8dfd4fa59cfabbd9098f9eb0cb4ba905d0"
          ...
    ```
1. Add a label to set the kube-image-deployer pod to watch.
    ```
    labels:
        kube-image-deployer: configured # for watching
    ```

# Pulumi transformation sample
```typescript
import { sync } from "cross-spawn";

const KUBE_IMAGE_DEPLOYER_ANNOTATION_PREFIX = "kube-image-deployer";

type ContainerLike = {
  image: string | Promise<string>;
  name: string;
};

const cache: Map<string, string> = new Map();

/**
 * Parsing kube-image-deployer annotations to update container.image to current image hash.
 */
export function transformationKubeImageDeployer(o: any) {
  const props = o?.props?.kind ? o.props : o;

  if (
    !props?.spec?.template?.spec?.containers?.length &&
    !props?.spec?.jobTemplate?.spec?.template?.spec?.containers?.length // for CronJob
  ) return o;

  const annotations = props.metadata.annotations || {};
  const containerImageConfigs = getKubeImageDeployerConfigFromAnnotations(annotations);

  if (!Object.keys(containerImageConfigs).length) return o;

  const spec = props.spec?.template?.spec
  const jobSpec = props.spec?.jobTemplate?.spec?.template?.spec;
  const containers = mixContainers(spec || jobSpec);

  let injected = false;

  for (const c of containers) {
    const containerImage = containerImageConfigs[c.name];
    if (!containerImageConfigs[c.name]) continue;

    c.image = getContainerImage(containerImage.url, containerImage.tag);
    injected = true;
  }

  if (injected) {
    props.metadata.labels = {
      // See this label in kube-image-deployer and register it as a watch target.
      "kube-image-deployer": "configured",
      ...props.metadata.labels,
    };
  }
}

/**
 * Combine containers and initContainers from Spec and return it
 */
function mixContainers(spec: {
  containers: ContainerLike[];
  initContainers?: ContainerLike[];
}) {
  return [...spec.containers, ...(spec.initContainers || [])];
}

function getKubeImageDeployerConfigFromAnnotations(annotations: {[key: string]: string;}) {
  const containerImages: {
    [key: string]: {
      containerName: string;
      url: string;
      tag: string;
    };
  } = {};

  for (const key of Object.keys(annotations)) {
    if (!key.startsWith(KUBE_IMAGE_DEPLOYER_ANNOTATION_PREFIX)) continue;

    const str = annotations[key];
    const [, containerName] = key.split("/", 2);
    const [url, tag] = str.split(":", 2);

    if (!containerName || !url || !tag) continue;

    containerImages[containerName] = { containerName, url, tag };
  }

  return containerImages;
}

function getContainerImage(url: string, tag: string): string {
  const cacheKey = `${url}:${tag}`;

  if (cache.has(cacheKey)) return cache.get(cacheKey) as string;

  const r = sync("kube-image-deployer-cli", ["--image", url, "--tag", tag], {
    env: {
      ...process.env,
      AWS_REGION: process.env.KUBE_IMAGE_DEPLOYER_AWS_REGION || process.env.AWS_REGION,
      AWS_PROFILE: process.env.KUBE_IMAGE_DEPLOYER_AWS_PROFILE || process.env.AWS_PROFILE,
    }
  });

  if (r.error) throw new Error(`getContainerImage error: ${url}:${tag} - ${r.error}`);
  if (r.stderr?.toString()) throw new Error(`getContainerImage error: ${url}:${tag} - ${r.stderr?.toString()}`);

  const result = r.output.join("").trim();

  if (!result.startsWith(url))
    throw new Error(`kube-image-deployer-cli unexprected result : ${url}:${tag} - ${result}`);

  cache.set(cacheKey, result);

  return result;
}
```
