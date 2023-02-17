To install kube-image-deployer in your Kubernetes cluster, you should deploy it as a StatefulSet.

This pod monitors workloads in your Kubernetes cluster that have the "kube-image-deployer" label set, and deploys new images to those workloads using Strategic Merge Patch when a new image is discovered. To enable this, the kube-image-deployer pod requires the appropriate Kubernetes service account and role binding.

In addition, the kube-image-deployer pod needs permission to read image tags from the Docker Registry you are using. To simplify the setup, you can inject a "dockerconfig" file into the Pod as a Volume. You can find sample YAML files for a Service Account, Cluster Role, Cluster Role Binding, StatefulSet, and Docker Access Secret at the provided links.

# Yaml Samples
- [Service Account](./yaml/service-account.yaml)
- [Cluster Role](./yaml/cluster-role.yaml)
- [Cluster Role Binding](./yaml/cluster-role-binding.yaml)
- [StatefulSet](./yaml/statefulset.yaml)
- [Docker Access Secret](./yaml/secrets.yaml)
  - Open the [yaml/secrets.yaml](./yaml/secrets.yaml) file and change the docker registry access permissions correctly.

To create the kube-image-deployer namespace, run the following command:
```bash
kubectl create namespace kube-image-deployer
```

Then, install all YAML files by running:
```bash
kubectl apply -f ./yaml
```

To check the status of kube-image-deployer, run:
```bash
kubectrl get pod -n kube-image-deployer
```

You should see output similar to:
```
NAME                    READY   STATUS    RESTARTS   AGE
kube-image-deployer-0   1/1     Running   0          0d
```
