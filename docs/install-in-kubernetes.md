# Install kube-image-deployer in your kubernetes cluster

Install kube-image-deployer as Statefulset. The kube-image-deployer pod monitors the kubernetes workload and performs a patch strategy when an image update is required.

For this, the kube-image-deployer pod needs the appropriate kubernetes service account and role binding.

kube-image-deployer pod need permission to read image tags from the Docker Registry you use, and to simplify the setup, here we will inject dockerconfig into the Pod as a Volume.

# Yaml Samples
- [Service Account](./yaml/service-account.yaml)
- [Cluster Role](./yaml/cluster-role.yaml)
- [Cluster Role Binding](./yaml/cluster-role-binding.yaml)
- [StatefulSet](./yaml/statefulset.yaml)
- [Docker Access Secret](./yaml/secrets.yaml)
  - Open the [yaml/secrets.yaml](./yaml/secrets.yaml) file and change the docker registry access permissions correctly.

# Create kube-image-deployer namespace
> kubectl create namespace kube-image-deployer

# Install all yaml files
> kubectl apply -f ./yaml

# Check the status of kube-image-deployer
> kubectrl get pod -n kube-image-deployer
```
NAME                    READY   STATUS    RESTARTS   AGE
kube-image-deployer-0   1/1     Running   0          0d
```
