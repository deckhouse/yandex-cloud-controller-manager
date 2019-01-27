# Kubernetes Cloud Controller Manager for Yandex.Cloud
[![Build Status](https://travis-ci.org/dlisin/yandex-cloud-controller-manager.svg?branch=master)](https://travis-ci.org/dlisin/yandex-cloud-controller-manager)
[![Go Report Card](https://goreportcard.com/badge/github.com/dlisin/yandex-cloud-controller-manager)](https://goreportcard.com/report/github.com/dlisin/yandex-cloud-controller-manager)
[![codecov](https://codecov.io/gh/dlisin/yandex-cloud-controller-manager/branch/master/graph/badge.svg)](https://codecov.io/gh/dlisin/yandex-cloud-controller-manager)
[![Docker Pulls](https://img.shields.io/docker/pulls/dlisin/yandex-cloud-controller-manager.svg)](https://hub.docker.com/r/dlisin/yandex-cloud-controller-manager/)

## Overview
`yandex-cloud-controller-manager` is the Kubernetes Cloud Controller Manager (CCM) implementation for Yandex.Cloud.  
It allows you to leverage many of the cloud provider features offered by Yandex.Cloud on your Kubernetes clusters.
Read more about Kubernetes CCM [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/). 

Currently `yandex-cloud-controller-manager` implements:
* `NodeController` - responsible for updating kubernetes nodes with cloud provider specific labels and addresses and deleting kubernetes nodes that were deleted on your cloud.

In the future, it may implement:
* `ServiceController` - responsible for creating LoadBalancers when a service of `Type: LoadBalancer` is created in Kubernetes.
* `RouteController` - responsible for creating firewall rules.


## Work In Progress
This project is currently under active development. Use at your own risk!
Contributions welcome!


## Getting Started

### Requirements
At the current state of Kubernetes, running Cloud Controller Manager (CCM) requires a few things.
Please read through the requirements carefully as they are critical to running CCM on a Kubernetes cluster on Yandex.Cloud.

#### Version
Kubernetes 1.11+

#### Cloud resources
* All Kubernetes nodes **MUST** be located in the same `Folder`.
For more details about folders - refer to official [documentation](https://cloud.yandex.ru/docs/resource-manager/concepts/resources-hierarchy)
* Kubernetes node names **MUST** match the VM name.
By default, the `kubelet` will name nodes based on the node hostname. On Yandex.Cloud, node hostname is set based on the VM name. 
So, it is important that the node name on Kubernetes matches corresponding VM name, otherwise CCM will not be able to find corresponding cloud resources.

#### Cluster configuration
* `kubelet` **MUST** run with `--cloud-provider=external`. 
This is to ensure that the `kubelet` is aware that it must be initialized by the CCM before it is scheduled any work.
* `kube-apiserver` and `kube-controller-manager` **MUST NOT** set the flag `--cloud-provider` which will default them to use no cloud provider natively.

**WARNING**: setting `--cloud-provider=external` will taint all nodes in a cluster with `node.cloudprovider.kubernetes.io/uninitialized`.
It is the responsibility of CCM to untaint those nodes once it has finished initializing them. 
This means that most pods will be left unschedulable until the CCM is running.

### Deployment

#### Authentication and Configuration
The `yandex-cloud-controller-manager` requires a API Access Token and the Folder ID stored in the following environment variables:
* `YANDEX_CLOUD_ACCESS_TOKEN`
* `YANDEX_CLOUD_FOLDER_ID`

The default manifest is configured to set these environment variables from a secret named `yandex-cloud`:

```bash
$ cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  labels:
    k8s-app: yandex-cloud-controller-manager
  name: yandex-cloud
  namespace: kube-system
stringData:
  access-token: "AQAAAAABf_abc123abc123abc123abc123abc123"
  folder-id: "b1g4c2a3g6vkffp3qacq"
EOF
```

#### Installation - with RBAC
```bash
kubectl apply -f manifests/yandex-cloud-controller-manager-rbac.yaml
kubectl apply -f manifests/yandex-cloud-controller-manager.yaml
```

#### Installation - without RBAC
```bash
kubectl apply -f manifests/yandex-cloud-controller-manager.yaml
```

**NOTE**: the deployments in `manifests/` folder are meant to serve as an example. 
They will work in a majority of cases but may not work out of the box for your cluster.


## Development
The `yandex-cloud-controller-manager` is written in Google's Go programming language. 
Currently, it is developed and tested on **Go 1.11.x**. 
If you haven't set up a Go development environment yet, please follow [these instructions](https://golang.org/doc/install).

### Download Source
```bash
$ go get -u github.com/dliin/yandex-cloud-controller-manager
$ cd $(go env GOPATH)/src/github.com/dliin/yandex-cloud-controller-manager
```

### Dependency management
`yandex-cloud-controller-manager` uses [Dep](https://github.com/golang/dep) to manage dependencies. 
Dependencies are already checked in the `vendor` folder. If you want to update/add dependencies, run:
```bash
$ make dep
```

### Build Binary
To build `yandex-cloud-controller-manager` binary, run:
```bash
$ make build
```

### Building Docker images
To build Docker image, use the following make target: 
```bash
$ DOCKER_TAG=dev make docker-build
```
