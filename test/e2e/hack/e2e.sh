#!/usr/bin/env bash
REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"
IMG_REPO="https://quay.io/api/v1/repository/redhat-user-workloads/crt-redhat-acm-tenant/maestro-main/maestro-main"

function wait_command() {
  local command="$1"; shift
  local wait_seconds="${1:-300}"; shift # 300 seconds as default timeout
  until [[ $((wait_seconds--)) -eq 0 ]] || eval "$command &> /dev/null" ; do sleep 1; done
  ((++wait_seconds))
}

broker_type=${BROKE_TYPE:-"kafka"}

latest_img_tag=$(curl -s -X GET "${IMG_REPO}" | jq -s -c -r 'sort_by(.tags[].last_modified) | .[].tags[].name' | grep -E '^[a-z0-9]{40}$' | head -n 1)

maestro_image=${MAESTRO_IMAGE_NAME:-"quay.io/redhat-user-workloads/crt-redhat-acm-tenant/maestro-main/maestro-main:${latest_img_tag}"}
maestro_addon_image=${IMAGE_NAME:-quay.io/stolostron/maestro-addon:latest}

echo "=== Maestro Image: $maestro_image"
echo "=== Maestro AddOn Image: $maestro_addon_image"

kind_cluster="maestro-addon-e2e-test"

cluster="loopback"

ocm_version="v0.15.0"
strimzi_version="0.44.0"

output="${REPO_DIR}/_output"
tools_dir="${output}/tools"

kube_config="${output}/kube.config"

kind="${tools_dir}/kind"
kubectl="${tools_dir}/kubectl"
helm="${tools_dir}/helm"
clusteradm="${tools_dir}/clusteradm"

mkdir -p ${tools_dir}

source ${REPO_DIR}/test/e2e/hack/deps.sh

echo "=== Prepare KinD test environment"
${kind} delete clusters ${kind_cluster}
${kind} create cluster --name ${kind_cluster} --kubeconfig ${kube_config}
${kind} load docker-image --name=${kind_cluster} ${maestro_addon_image}

export KUBECONFIG=${kube_config}

if [ $broker_type = "kafka" ]; then
  echo "=== Prepare Kafka cluster"
  wget -O $output/strimzi-cluster-operator-${strimzi_version}.yaml https://github.com/strimzi/strimzi-kafka-operator/releases/download/${strimzi_version}/strimzi-cluster-operator-${strimzi_version}.yaml
  sed -i 's/myproject/strimzi/' $output/strimzi-cluster-operator-${strimzi_version}.yaml
  ${kubectl} create namespace strimzi
  ${kubectl} -n strimzi create -f $output/strimzi-cluster-operator-${strimzi_version}.yaml
  wait_command "${kubectl} -n strimzi get deploy strimzi-cluster-operator"
  ${kubectl} -n strimzi wait deploy/strimzi-cluster-operator --for=condition=Available --timeout=300s
  ${kubectl} -n strimzi create -f ${REPO_DIR}/test/e2e/hack/manifests/kafka.yaml
  ${kubectl} -n strimzi wait kafka/kafka --for=condition=Ready --timeout=300s
fi

echo "=== Prepare OCM hub"
${clusteradm} init --wait --bundle-version="${ocm_version}" \
  --feature-gates=CloudEventsDrivers=true,ManifestWorkReplicaSet=true \
  --output-join-command-file="${output}/join.sh"

echo "=== Deploy Maestro AddOn on the hub"
${helm} install maestro-addon charts/maestro-addon \
  --set maestro.broker=${broker_type} \
  --set global.imageOverrides.maestroImage=${maestro_image} \
  --set global.imageOverrides.maestroAddOnImage=${maestro_addon_image} \
  --set messageQueue.grpc.type=internal \
  --set messageQueue.amqStreams.namespace=strimzi \
  --set messageQueue.amqStreams.listener.type=internal \
  --set messageQueue.amqStreams.listener.port=9093
${kubectl} -n maestro wait deploy/maestro --for=condition=Available --timeout=300s
${kubectl} -n maestro wait deploy/maestro-addon-manager --for=condition=Available --timeout=300s

echo "=== Join a cluster to the hub"
sh -c "$(cat ${output}/join.sh) ${cluster} --force-internal-endpoint-lookup"
${clusteradm} accept --wait --clusters ${cluster}

echo "=== Deploy Maestro Agent with ManagedClusterAddOn on the managed cluster"
${kubectl} -n ${cluster} apply -f ${REPO_DIR}/test/e2e/hack/manifests/managedclusteraddon.yaml
${kubectl} -n ${cluster} wait managedclusteraddons/maestro-addon --for=condition=Available --timeout=300s

echo "=== Enable ManifestWorkReplicaSet controller on the hub"
${kubectl} -n open-cluster-management create -f ${REPO_DIR}/test/e2e/hack/manifests/work-driver-config.secret.yaml
${kubectl} patch clustermanager cluster-manager -p='{"spec":{"workConfiguration":{"workDriver":"grpc"}}}' --type=merge
${kubectl} -n open-cluster-management-hub wait deploy/cluster-manager-work-controller --for=jsonpath='{.status.replicas}'=2 --timeout=300s
${kubectl} -n open-cluster-management-hub wait deploy/cluster-manager-work-controller --for=jsonpath='{.status.replicas}'=1 --timeout=300s
${kubectl} -n open-cluster-management-hub wait deploy/cluster-manager-work-controller --for=condition=Available --timeout=300s

echo "=== Prepare the placement on the hub"
${kubectl} apply -f ${REPO_DIR}/test/e2e/hack/manifests/managedclustersetbinding.yaml
${kubectl} apply -f ${REPO_DIR}/test/e2e/hack/manifests/placement.yaml

${output}/e2e.test -test.v -ginkgo.v
