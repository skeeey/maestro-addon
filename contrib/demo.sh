#!/usr/bin/env bash

CURRENT_DIR="$(dirname "${BASH_SOURCE[0]}")"
REPO_DIR="$(cd ${CURRENT_DIR} && cd .. && pwd)"
WORK_DIR="$(cd ${CURRENT_DIR} && pwd)"

source ${WORK_DIR}/demo_magic

cluster_name=${CLUSTER_NAME:-local-cluster}

kafka_cluster_namespace=${KAFKA_CLUSTER_NAMESPACE:-amq-streams}
kafka_cluster_name=${KAFKA_CLUSTER_NAME:-kafka}

helm_args=""

if [ "$kafka_cluster_namespace" != "amq-streams" ]; then
    helm_args="--set messageQueue.amqStreams.namespace=${kafka_cluster_namespace}"
fi

if [ "$kafka_cluster_name" != "kafka" ]; then
    helm_args="${helm_args} --set messageQueue.amqStreams.name=${kafka_cluster_name}"
fi

# check env
echo "oc get mce"
oc get mce
if [ "$?" != "0" ]; then
    exit 1
fi

echo "oc get managedclusters ${cluster_name}"
oc get managedclusters ${cluster_name}
if [ "$?" != "0" ]; then
    exit 1
fi

echo "oc -n ${kafka_cluster_namespace} get kafkas ${kafka_cluster_name}"
oc -n ${kafka_cluster_namespace} get kafkas ${kafka_cluster_name}
if [ "$?" != "0" ]; then
    exit 1
fi

# start the demo
comment "Run helm command to install maestro addon on the hub"
pe "helm install maestro-addon ${REPO_DIR}/charts/maestro-addon ${helm_args}"
comment "Wait for the maestro addon to be deployed on the hub"
pe "oc -n maestro wait deploy/maestro --for=condition=Available --timeout=300s"
comment "There are three componetnts are running, including: maestro-addon-manager, PostgreSQL database and maestro"
comment "sever and the ClusterManagementAddon maestro-addon is also created"
pe "oc -n maestro get pods"
pe "oc get clustermanagementaddons.addon.open-cluster-management.io"

comment "The managed cluster ${cluster_name} will be created as a consumer in the maestro database"
db_pod_name=$(oc -n maestro get pods -l name=maestro-db -ojsonpath='{.items[0].metadata.name}')
pe "oc -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select * from consumers'"

comment "Create a ManagedClusterAddon to install the maestro addon agent on the managed cluster ${cluster_name}"
pe "oc -n ${cluster_name} apply -f ${WORK_DIR}/examples/managedclusteraddon.yaml"
comment "Wait for the ManagedClusterAddon to be available"
pe "oc -n ${cluster_name} wait managedclusteraddons/maestro-addon --for=condition=Available --timeout=300s"
comment "The maestro addon agent is running on the managed cluster ${cluster_name}"
pe "oc -n open-cluster-management-agent get pods"

comment "Create a config for manifestworkreplicaset controller to connect the maestro server with GRPC"
pe "oc -n multicluster-engine create -f ${WORK_DIR}/examples/work-driver-config.yaml"
pe "oc -n multicluster-engine get secrets work-driver-config -ojsonpath='{.data.config\.yaml}' | base64 -d"

comment "Enable manifestworkreplicaset controller on the hub"
pe "oc patch clustermanager cluster-manager --type=merge -p='{\"spec\":{\"workConfiguration\":{\"workDriver\":\"grpc\",\"featureGates\":[{\"feature\":\"CloudEventsDrivers\",\"mode\":\"Enable\"},{\"feature\":\"ManifestWorkReplicaSet\",\"mode\":\"Enable\"}]}}}'"
comment "Wait for the manifestworkreplicaset controller to be enabled"
pe "oc -n open-cluster-management-hub wait deploy/cluster-manager-work-controller --for=condition=Available --timeout=300s"

comment "Prepare a placement for ManifestWorkReplicaSet"
pe "oc apply -f ${WORK_DIR}/examples/placement"
comment "Apply a ManifestWorkReplicaSet to deploy a busybox to the managed cluster ${cluster_name}"
pe "oc apply -f ${WORK_DIR}/examples/manifestworkreplicaset.yaml"
comment "Wait for the ManifestWorkReplicaSet to be available"
pe "oc get manifestworkreplicasets busybox -w"
comment "The workloads of this manifestworkreplicaset is applied on the managed cluster ${cluster_name}"
pe "oc -n mwrs-test get deployments.apps"

comment "There is no ManifestWork CR for the workloads on the hub as we use the maestro to transport the workload and"
comment " its status with cloudevents"
pe "oc -n ${cluster_name} get manifestworks -l work.open-cluster-management.io/manifestworkreplicaset=default.busybox"
pe "oc -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select jsonb_pretty(payload) from resources'"
pe "oc -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select jsonb_pretty(status) from resources'"
