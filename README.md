# Kubernetes Cloud Controller Manager for Yandex.Cloud
![Actions Status](https://github.com/deckhouse/yandex-cloud-controller-manager/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/deckhouse/yandex-cloud-controller-manager)](https://goreportcard.com/report/github.com/deckhouse/yandex-cloud-controller-manager)
[![codecov](https://codecov.io/gh/deckhouse/yandex-cloud-controller-manager/branch/master/graph/badge.svg)](https://codecov.io/gh/deckhouse/yandex-cloud-controller-manager)

## Overview
`yandex-cloud-controller-manager` is the Kubernetes Cloud Controller Manager (CCM) implementation for Yandex.Cloud.
It allows you to leverage many of the cloud provider features offered by Yandex.Cloud on your Kubernetes clusters.
Read more about Kubernetes CCM [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

Currently `yandex-cloud-controller-manager` implements:
* `NodeController` - responsible for updating kubernetes nodes with cloud provider specific labels and addresses and deleting kubernetes nodes that were deleted on your cloud.
* `ServiceController` - responsible for creating LoadBalancers when a service of `Type: LoadBalancer` is created in Kubernetes.
* `RouteController` - responsible for creating routes to allow networking between Pods.


## Work In Progress
This project is currently under active development. Use at your own risk!
Contributions welcome!


## Getting Started

### Requirements
At the current state of Kubernetes, running Cloud Controller Manager (CCM) requires a few things.
Please read through the requirements carefully as they are critical to running CCM on a Kubernetes cluster on Yandex.Cloud.

#### Version
Kubernetes 1.16+

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

**Caution**: Yandex.Cloud's documentation refers to **Network** or **VPC** interchangeably. If a **Subnet** ID is required in the configuration, it will be explicitly stated.

#### Authentication and Configuration
The `yandex-cloud-controller-manager` requires a [Service Account Json]([https://cloud.yandex.com/docs/iam/operations/iam-token/create-for-sa#via-cli]), Folder ID and a Cloud-unique cluster Name stored in the following environment variables:
* `YANDEX_CLOUD_SERVICE_ACCOUNT_JSON`
* `YANDEX_CLOUD_FOLDER_ID`
* `YANDEX_CLUSTER_NAME`

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
  service-account-json: |
    {
       "id": "ajesh3orip69r0vctpf5",
       "service_account_id": "aje3qbblkdf2u2avn4n7",
       "created_at": "2020-01-20T07:43:49Z",
       "key_algorithm": "RSA_2048",
       "public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAozah4aqf9xkMLRRtNJjz\nZ+xooLV6GLGaHbkl3796r2zWbgm1aNU3xILGeStdTM5XbB651OAfq7YyHoDSiZkj\nBP6W2ZYNO/WjK9N13rWhtFjNiDulLh3LAU47qNy75OsC3SjW58MVHPNriIgB0HLA\nHRE6cguUJtUcKWqOKhoKQVJxXLOhsdjmEEdnHtd9ro3UCcPM7r6fc+MmkCaZgTNl\nkItkDDxodTIqj3Go2EiEyO2DaMye+GqTzSNJOYaHFH4DYhCCgE1/SCY356nER2qH\nymbAGkD2fAp2eGoCEM67AgqrWFEm/yI+FlIvcrn7wC6+NfWUVqPb+N4wuiehCRBO\n8wIDAQAB\n-----END PUBLIC KEY-----\n",
       "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCjNqHhqp/3GQwt\nFG00mPNn7GigtXoYsZoduSXfv3qvbNZuCbVo1TfEgsZ5K11MzldsHrnU4B+rtjIe\ngNKJmSME/pbZlg079aMr03XetaG0WM2IO6UuHcsBTjuo3Lvk6wLdKNbnwxUc82uI\niAHQcsAdETpyC5Qm1Rwpao4qGgpBUnFcs6Gx2OYQR2ce132ujdQJw8zuvp9z4yaQ\nJpmBM2WQi2QMPGh1MiqPcajYSITI7YNozJ74apPNI0k5hocUfgNiEIKATX9IJjfn\nqcRHaofKZsAaQPZ8CnZ4agIQzrsCCqtYUSb/Ij4WUi9yufvALr419ZRWo9v43jC6\nJ6EJEE7zAgMBAAECggEAF0hi3XNesHw1PXUNgxRSnL+fyVU6Hq2vQ5A28+03zjCj\ngj0GUPchpnnVYFGsVJmW5QiZD+INAozSJ4HPBuv+j+bVlCKQrr4C0eyvgt68O6Lz\nZvzDOonrfLsxTYx3jVdtKCl8RsGQkHm1HFvyjk7gUwUzJjO6pbN++fWGZEEkt16W\nFHaGldz2MvZKOwQwj0WUcjF4X8PWTvJ0Ar1i5XoAm35GN+2ziwJKcNt+DsJ3N6MW\ngAkivYE8f44T3fQFg5M1RC6v2Jp2lmtVRxYRI0rcie+HyxJVcHgWTZPdTkwGWKDD\nnRU2OTJoZCJf3BunFtB1P8W+GlmLFdBjTppFhgqI6QKBgQDO61fX49qVRDmORYor\nVWh1tZkw546llwkNqLAe1QoLhqjHGctUs3lOczDqI82PwGKb423JdgKmr9nzrCZW\nq5a/BulHdsGsvkSBGK091fhQYRQnaTF7bnXoyVI8qUerGiV/a8/7W1SM+WIJayZ3\nr5Z9xV9LH/Wy7uWW7Xr2LvP97wKBgQDJ7VuYZVJ17hPIqEaR3P1Jvka+RvusWTPw\n1o6O935tW28Q/S2J661Dv92mTNmll/beyFS1ZkHdQ/1c/92Pr1bM/A4CrQoNDvad\nhd2MnyzVYyHc4p6Yd6VmZisbPpTfa7ZJMzYUN27nj+yPxJyZ/EoLlcXaXcPV3kFo\nZsubNT0DPQKBgQCz/nLmgPWWnMd4ZDOB6IS6yCKfMP6cOtsMP64c0/Mt/ZB5yY1f\ne9PNE1T8h/J71r2wn1DUS8yYlSYB2sFq6U5zk55/pOVq0AQlTIL+5E9iFGCEu/Po\nTDlTKzVXQWXviAoQYoeEPnk5PII0cToAKQS/GV8AqaeAZGHhPWmWF1f1jwKBgHQx\nJ6aejv+bGjk5Uzo1rm3TloOA9uqqfa/U1j0//rjQhy2AccbOHWpBqjo6OHcH5Z82\nKUAkcjvvFoiAFq7KVykm1K0HgyQWeyQTVnPHWBYFsAOZR2c2Wa99lMpdjW6uXTrr\nw++IIkIO2DG2EeKtgLH/4dSQZdLXzE1V8U0DKnOFAoGAQNCBpnE1WHR9H5APr5SF\nuD35dTm3O2OvczlbB0MUhx8R7qPpvLwA5HRSIKAKxobUbGpdgCy6WuncRWg+TjaD\n8zlwZvG2+vtntCFPcIT6ZpGH6k9ppXOPJBxaHZRHJSoilGhF1KvrmY8T5WxTVuyM\nnmypFU40LHcTmvw/a6JY+BM=\n-----END PRIVATE KEY-----\n"
    }
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

**NOTE**: the deployments in `manifests` folder are meant to serve as an example.
They will work in a majority of cases but may not work out of the box for your cluster.

### Subsystem-specific information

#### Node Controller

##### CCM environment variables

* `YANDEX_CLOUD_INTERNAL_NETWORK_IDS` – comma separated list of NetworkIDs. Will be used to select InternalIPs when scanning an Yandex Instance and populating the corresponding Kubernetes Node.
    * Optional.
    * If **present**, we iterate over all Instance's interfaces and select networkID-matching addresses.
    * If **not present**, we use an IP address from the first interface as an InternalIP.
* `YANDEX_CLOUD_EXTERNAL_NETWORK_IDS` – comma separated list of NetworkIDs. Will be used to select ExternalIPs when scanning an Yandex Instance and populating the corresponding Kubernetes Node.
    * Optional.
    * If **present**, we iterate over all Instance's interfaces and select networkID-matching *private* addresses.
    * If **not present**, we use *public* address from the first interface that has one-to-one NAT enabled, or none at all.

#### Service Controller

##### Operation peculiarities

Due to Yandex.Cloud's TargetGroup's inability to create duplicate (same SubnetID and IP address) Targets, this CCM takes all Nodes in the clusters, finds its Yandex.Cloud Instance counterpart via the API, scans the Instance's Interfaces and determines all possible sets of NetworkIDs that exist on these instances. After doing so, it creates multiple TargetGroups that are named after aforementioned NetworkIDs and include Targets from said Networks.

Due to API limitations, only one subnet from each zone must be present in each NetworkID present on Instance's network interfaces.

##### CCM environment variables

* `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID` – default NetworkID to use for TargetGroup for created NetworkLoadBalancers.
    * Mandatory.
* `YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID` – default SubnetID to use for created NetworkLoadBalancers' listeners.
    * **Caution!** All newly created NLBs will be INTERNAL. This can be overriden via `yandex.cpi.flant.com/loadbalancer-external` [Service annotation](#Service-annotations).

##### Service annotations

* `yandex.cpi.flant.com/target-group-network-id` – override `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID` on a per-service basis.
* `yandex.cpi.flant.com/listener-subnet-id` – default SubnetID to use for Listeners in created NetworkLoadBalancers. NetworkLoadBalancers will be INTERNAL.
* `yandex.cpi.flant.com/listener-address-ipv4` – select pre-defined IPv4 address. Works both on internal and external NetworkLoadBalancers.
* `yandex.cpi.flant.com/loadbalancer-external` – override `YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID` per-service.
* `yandex.cpi.flant.com/target-group-name-prefix` - set target group for LB to target group with name `yandex.cpi.flant.com/target-group-name-prefix` annotation value + yandex cluster name + `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID`.

##### Node annotations

* `yandex.cpi.flant.com/target-group-name-prefix` - set node to the non-default target group add this annotation to the node.  Yandex CCM creates new target groups with name `yandex.cpi.flant.com/target-group-name-prefix` annotation value + yandex cluster name + network id of instance interfaces.

## Warning

1. If masters are created with their own target groups, then you need to attach the `node.kubernetes.io/exclude-from-external-load-balancers: ""` label on them so that the controller does not try to add the master to a new target group for balancers

2. If you want to add a balancer for the masters yourself, so that ycc could also hang its own balancers on the masters, create a target group in advance by the name mask `${CLUSTER-NAME}${VPC.ID}`

## Development
The `yandex-cloud-controller-manager` is written in Google's Go programming language.
Currently, it is developed and tested on **Go 1.13.6**.
If you haven't set up a Go development environment yet, please follow [these instructions](https://golang.org/doc/install).

### Download Source
```bash
$ go get -u github.com/deckhouse/yandex-cloud-controller-manager
$ cd $(go env GOPATH)/src/github.com/deckhouse/yandex-cloud-controller-manager
```

### Dependency management
`yandex-cloud-controller-manager` uses [Go modules](https://github.com/golang/go/wiki/Modules) to manage dependencies.

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
