# webapp-operator

Simple operator to handle Web Application like deployments

## Description

This operator helps you to deploy a Web application with multiple replicas on an HTTPS based host.

### Installation 

In this section you'll learn, how to install the operator and your application as a Custom Resource into a Kubernetes cluster.

### Prerequisite

In order to deploy the operator and your web application, you'll need to have the following step done.

- Kubernetes cluster with the [NGINX Ingerss Controller](https://github.com/kubernetes/ingress-nginx).

  Please refer to the [NGINX Ingerss Controller](https://github.com/kubernetes/ingress-nginx) documentation about how to install it into your cluster.
  Here is a simplified version for installing the Nginx Ingress Controller.

  Please note: your version might different from the example (v1.6.4).
  ```sh
  kubectl \
     apply \
     -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.6.4/deploy/static/provider/cloud/deploy.yaml
  ```

- Kubernetes cluster with the [cert-manager](https://github.com/cert-manager/cert-manager).

  Please refer to the [cert-manager installation](https://cert-manager.io/docs/installation/kubectl/) documentation about how to install it into your cluster.
  Here is a simplified version for installing the cert-manager.

  Please note: your version might different from the example (v1.11.0).
  ```sh
  kubectl \
     apply \
     -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
  ```

- You'll need a [ACME type ClusterIssuer](https://cert-manager.io/docs/configuration/acme/) to be deployed into your cluster.

  In-order to issue certificates to your hostname you'll need to install a ClusterIssuer.
  A simplified manifiest file, based on the Let's Encrypt provider might look the following. 
  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt-staging
  spec:
    acme:
      email: webapp-test@gmail.com
      server: https://acme-staging-v02.api.letsencrypt.org/directory
      privateKeySecretRef:
        name: webapp-account-key
      solvers:
       - http01:
         ingress:
           class: nginx
  ```
  Save this file to `lets-encrypt-staging.yaml` and apply it with the `kubectl` command to your cluster.

  ```sh
  kubectl \
     apply \
     -f lets-encrypt-staging.yaml
  ```
- To be able to lookup to the hostname you choose to host your Web Application we assume you know how to set up the DNS in your environment.

  In some local environment this can be done by editing the hostfile (`/etc/hosts`, `C:\Windows\System32\drivers\etc\hosts`).

- The [operator image](https://hub.docker.com/r/vcsomor/webapp-operator/tags) is hosted on the docker-hub, please make sure you cluster has pull rights to that registry. 

After installing the required components please verify them.
You should have the `cert-manager` deployments running in the `cert-manager` namespace and the `ingress-nginx-controller` deployment running in th `ingress-nginx` namespace.

### Installation of the Operator

You simply install the `webapp-operator` by applying the [webapp-operator.yaml](config/webapp-operator.yaml) to your cluster.

```sh
kubectl \
 apply \
  -f https://raw.githubusercontent.com/vcsomor/webapp-operator/main/config/webapp-operator.yaml
```

After the operator has been installed, you can create your webapp CR in your cluster.
The Webapp custom resource has the below properties.
- `image` the docker image of your web application (e.g.: `nginx:latest`)
- `containerPort` the port exposed inside your `image`
- `replicas` defines the number of the replica count of your applications 
- `host` defines the host of the application 
- `cert-manager` the name of the `ClusterIssuer` resource that is intended to issue TLS certificates to the `host` domain name.

Example custom resources can be found [here](config/samples).

Webapp custom resource example.
```yaml
apiVersion: webapp.viq.io/v1alpha1
kind: Webapp
metadata:
  labels:
    app.kubernetes.io/name: webapp
    app.kubernetes.io/instance: webapp-sample
    app.kubernetes.io/part-of: webapp-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: webapp-operator
  name: webapp-sample
spec:
  replicas: 3
  host: webapp.yourdomain.com
  image: nginx:latest
  containerPort: 80
  cert-manager: letsencrypt-staging
```

Apply your CR manifest file to your cluster.

```sh
kubectl \
 apply \
  -f webapp-sample.yaml
```

After applying the resource you should see a `Deployment`, `Service` and an `Ingress` created with a `webapp-sample` name in your cluster.

If your DNS configurations are correct, the application should be reachable on the given `host` (e.g.: `webapp.yourdomain.com`). 

```sh
# Assuming your DNS configurations are correct.
# The '-k' switch in the curl command is only need if you havent imported the certificate into your certificate store.
curl -k https://webapp.yourdomain.com
```

Sample response:
```
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
... omitted ...
```

## Developer Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/webapp-operator:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/webapp-operator:tag
```

4. Package the final kubernetes install yaml file:

```sh
make package
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
No contribution is intended.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
