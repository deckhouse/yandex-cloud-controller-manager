# cloud provisioning

```bash
export YANDEX_SA_DATA=$(yc iam service-account create --name k8s-cloud-controller --format json )
export YANDEX_SA_ID=$(echo $YANDEX_SA_DATA | jq .id | sed 's\"\\g')
export YANDEX_SA_FOLDER=$(echo $YANDEX_SA_DATA | jq .folder_id | sed 's\"\\g' | tr -d '\n' )
export YANDEX_SA_FOLDER_B64=$(echo -n $YANDEX_SA_FOLDER | base64 )


yc resource-manager folder add-access-binding ${YANDEX_SA_FOLDER} \
  --role admin \
  --subject serviceAccount:${YANDEX_SA_ID}

yc iam key create --service-account-name k8s-cloud-controller  --output k8s-cloud-controller-key.json

export serviceAccountJSON=$( base64 k8s-cloud-controller-key.json | tr -d '\n')

export NAMESPACE=kube-system

# K8S_LB_IP and K8S_LB_PORT for API access in hostNetwork without cni (without HTTPS/HTTP)
export K8S_LB_IP="51.250.93.44"
export K8S_LB_PORT="443"

# podCIDR to change the route table in the cloud
export podCIDR="100.0.0.0/24"

# clusterName for points in the cloud
export clusterName="cluster-2"

# vpcID - id of the vpc on which you are setting up the cluster
export vpcID="enpmp9gdka3l65ous1lv"

# routeTableID - id of the route table that is connected to the vpc
export routeTableID="enpmamolbm2cld5mj26a"

helm upgrade yandex-cloud-controller . \
--install \
--create-namespace \
--namespace=${NAMESPACE} \
--set=serviceAccountJSON=${serviceAccountJSON} \
--set=folderID=${YANDEX_SA_FOLDER_B64} \
--set=vpcID=${vpcID} \
--set=routeTableID=${routeTableID} \
--set=k8sApiServer=${K8S_LB_IP} \
--set=k8sApiServerPort=${K8S_LB_PORT} \
--set=podCIDR=${podCIDR} \
--set=clusterName=${clusterName}
```