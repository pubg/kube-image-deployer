# Using with Pulumi
In our case, we used [Pulumi](https://pulumi.com) as our Kubernetes IaC. It explains how to integrate with Pulumi, which was needed at this time.

### [Pulumi Transformation](https://www.pulumi.com/docs/intro/concepts/resources/options/transformations/) 
Here are the steps for using Pulumi transformations to avoid redeploying older container images during a pulumi up operation:

1. During a `pulumi up`, the container image might already be up-to-date because of previous CI/CD pipeline runs.
1. To avoid redeploying an older container image, it's necessary to retrieve the hash of the currently deployed image and pass it to `pulumi up`.
1. This ensures that the same container image is used in the new deployment, preventing the unnecessary redistribution of older images.

### How to use
Here are the instructions with improvements for the 3 steps you provided:
1. Verify that the annotation rule for kube-image-deployer is configured correctly in Pulumi transformations:
    ```yaml
    kube-image-deployer/[container.name]: '[container.image:tagExpr]'
    ```
    If you want to deploy a container image like the example below:
    ```yaml
    containers:
        - name: main
          image: busybox:1.31.*
          ...
    ```
    Set the annotation like this:
    ```yaml
    annotations:
        kube-image-deployer/main: 'busybox:1.31.*'
    ```

1. Extract the latest hash of the container image from the annotation and set it on the container image. Update the containers section in the YAML file to use the retrieved hash as shown below:
    ```yaml
    containers:
    - name: main
      image: "busybox@sha256:fd4a8673d0344c3a7f427fe4440d4b8dfd4fa59cfabbd9098f9eb0cb4ba905d0"
      ...
    ```
1. Add a label to enable the kube-image-deployer pod to watch for changes. Update the YAML file by adding the label as shown below:
    ```yaml
    labels:
        kube-image-deployer: configured # for watching
    ```

# Pulumi transformation example
```typescript
import { sync } from "cross-spawn";

const KUBE_IMAGE_DEPLOYER_ANNOTATION_PREFIX = "kube-image-deployer";

type ContainerLike = {
  image: string | Promise<string>;
  name: string;
};

const cache: Map<string, string> = new Map();

/**
 * This is a function that takes a Kubernetes object and transforms it by
 * injecting container images specified in the object's metadata annotations
 * using kube-image-deployer.
 */
export function transformationKubeImageDeployer(o: any) {
  // Extract the props from the input object, handling nested props if they exist.
  const props = o?.props?.kind ? o.props : o;

  // Check if the object has containers or jobs, and if not, return the original object.
  if (
    !props?.spec?.template?.spec?.containers?.length &&
    !props?.spec?.jobTemplate?.spec?.template?.spec?.containers?.length // for CronJob
  ) return o;

  // Get the annotations from the object's metadata, or an empty object if none exist.
  const annotations = props.metadata.annotations || {};
  // Extract the container image configurations from the annotations.
  const containerImageConfigs = getKubeImageDeployerConfigFromAnnotations(annotations);

  // If there are no container image configurations, return the original object.
  if (!Object.keys(containerImageConfigs).length) return o;

  // Extract the containers from the object's spec, depending on the type of object.
  const spec = props.spec?.template?.spec
  const jobSpec = props.spec?.jobTemplate?.spec?.template?.spec;
  const containers = mixContainers(spec || jobSpec);

  // Initialize a flag to track whether any containers were modified.
  let injected = false;

  // Loop through each container and check if there is a corresponding container image
  // configuration in the annotations. If so, inject the container image and update the flag.
  for (const c of containers) {
    const containerImage = containerImageConfigs[c.name];
    if (!containerImageConfigs[c.name]) continue;

    c.image = getContainerImage(containerImage.url, containerImage.tag);
    injected = true;
  }

  // If any containers were modified, update the object's metadata labels to indicate
  // that kube-image-deployer has been configured.
  if (injected) {
    props.metadata.labels = {
      // See this label in kube-image-deployer and register it as a watch target.
      "kube-image-deployer": "configured",
      ...props.metadata.labels,
    };
  }
}

/**
 * This is a function that takes a Kubernetes object spec containing both
 * regular and init containers, and returns an array containing all containers.
 */
function mixContainers(spec: {
  containers: ContainerLike[];
  initContainers?: ContainerLike[];
}) {
  return [...spec.containers, ...(spec.initContainers || [])];
}

/**
 * This is a function that extracts container image configurations from an object's
 * metadata annotations. It looks for annotations that start with the KUBE_IMAGE_DEPLOYER_ANNOTATION_PREFIX,
 * and if it finds one, it extracts the container name, URL, and tag from the annotation
 * and adds it to a dictionary of container image configurations.
 */
function getKubeImageDeployerConfigFromAnnotations(annotations: {[key: string]: string;}) {
  // Initialize an empty dictionary to hold the container image configurations.
  const containerImages: {
    [key: string]: {
      containerName: string;
      url: string;
      tag: string;
    };
  } = {};

  // Loop through each annotation and check if it starts with the prefix.
  for (const key of Object.keys(annotations)) {
    if (!key.startsWith(KUBE_IMAGE_DEPLOYER_ANNOTATION_PREFIX)) continue;

    const str = annotations[key];
    const [, containerName] = key.split("/", 2);
    const [url, tag] = str.split(":", 2);

    if (!containerName || !url || !tag) continue;

    containerImages[containerName] = { containerName, url, tag };
  }

  // Return the dictionary of container image configurations.
  return containerImages;
}

/**
 * This is a function that uses the kube-image-deployer-cli to retrieve a container image.
 * The function first checks if the image is in the cache, and if so, returns it.
 * Otherwise, it calls the kube-image-deployer-cli with the provided URL and tag to
 * retrieve the image. The function also sets the AWS_REGION and AWS_PROFILE environment
 * variables as needed, and throws an error if there is a problem retrieving the image.
 */
function getContainerImage(url: string, tag: string): string {
  // Create a cache key from the URL and tag.
  const cacheKey = `${url}:${tag}`;

  // Check if the image is in the cache, and if so, return it.
  if (cache.has(cacheKey)) return cache.get(cacheKey) as string;

  // Call the kube-image-deployer-cli to retrieve the image hash.
  const r = sync("kube-image-deployer-cli", ["--image", url, "--tag", tag], {
    env: {
      ...process.env,
      AWS_REGION: process.env.KUBE_IMAGE_DEPLOYER_AWS_REGION || process.env.AWS_REGION,
      AWS_PROFILE: process.env.KUBE_IMAGE_DEPLOYER_AWS_PROFILE || process.env.AWS_PROFILE,
    }
  });

  // If there is an error, throw an error message.
  if (r.error) throw new Error(`getContainerImage error: ${url}:${tag} - ${r.error}`);
  // If there is stderr output, throw an error message.
  if (r.stderr?.toString()) throw new Error(`getContainerImage error: ${url}:${tag} - ${r.stderr?.toString()}`);

  // Get the output from the command and trim any whitespace.
  const result = r.output.join("").trim();

  // Check that the output starts with the URL, and throw an error if it doesn't.
  if (!result.startsWith(url))
    throw new Error(`kube-image-deployer-cli unexprected result : ${url}:${tag} - ${result}`);

  // Add the image to the cache and return it.
  cache.set(cacheKey, result);

  return result;
}
```
