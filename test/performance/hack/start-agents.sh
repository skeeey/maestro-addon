#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

HUB_KUBECONFIG="/Users/liuwei/kubes/awsocp.kubeconfig"

total=${total:-1}
begin_index=${begin_index:-1}
cluster_with_works=${cluster_with_works:-"false"}

# total number of agents created at one time
counts=1

# work dir
crds_dir=${REPO_DIR}/test/performance/hack/crds
work_dir=${REPO_DIR}/_output/performance
spoke_kube_dir=${work_dir}/clusters
kafka_cert_dir=${work_dir}/kafka
agent_config_dir=${work_dir}/agentconfigs
agent_log_dir=${work_dir}/logs
pid_dir=${work_dir}/pids

mkdir -p ${spoke_kube_dir}
mkdir -p ${kafka_cert_dir}
mkdir -p ${agent_config_dir}
mkdir -p ${agent_log_dir}
mkdir -p ${pid_dir}

echo "Start agents (start=$begin_index, total=$total) ..."

# get kafka cluster host and certs
kafka_host=$(kubectl --kubeconfig ${HUB_KUBECONFIG} -n amq-streams get route kafka-kafka-tls-bootstrap -ojsonpath='{.spec.host}')
kubectl --kubeconfig ${HUB_KUBECONFIG} -n amq-streams get secrets kafka-cluster-ca-cert -ojsonpath="{.data.ca\.crt}" | base64 -d > ${kafka_cert_dir}/cluster-ca.crt
kubectl --kubeconfig ${HUB_KUBECONFIG} -n amq-streams get secrets kafka-clients-ca-cert -ojsonpath="{.data.ca\.crt}" | base64 -d > ${kafka_cert_dir}/clients-ca.crt
kubectl --kubeconfig ${HUB_KUBECONFIG} -n amq-streams get secrets kafka-clients-ca -ojsonpath="{.data.ca\.key}" | base64 -d > ${kafka_cert_dir}/clients-ca.key

# prepare kind clusters
kind_clusters=$(($total/$counts))
kind_index=0
while ((kind_index<kind_clusters))
do
    echo "Start kind cluster test-$kind_index ..."
    kind create cluster --name "test-$kind_index" --kubeconfig "${spoke_kube_dir}/test-${kind_index}.kubeconfig"
    kubectl --kubeconfig ${spoke_kube_dir}/test-${kind_index}.kubeconfig apply -f ${crds_dir}
    kind_index=$(($kind_index + 1))
done

# start agents
agent_total=0
kind_index=0
index=$begin_index
while ((agent_total<total))
do
    lastIndex=$(($index + $counts - 1))
    echo "Start agents for cluters [$index, $lastIndex] ..."
    echo "The kind cluster ${spoke_kube_dir}/test-${kind_index}.kubeconfig is used"

    args="--agent-config-dir=${agent_config_dir}"
    args="${args} --kafka-server=${kafka_host}"
    args="${args} --kafka-cluster-ca=${kafka_cert_dir}/cluster-ca.crt"
    args="${args} --kafka-client-ca=${kafka_cert_dir}/clients-ca.crt"
    args="${args} --kafka-client-ca-key=${kafka_cert_dir}/clients-ca.key"
    args="${args} --hub-kubeconfig=${HUB_KUBECONFIG}"
    args="${args} --spoke-kubeconfig=${spoke_kube_dir}/test-${kind_index}.kubeconfig"
    args="${args} --cluster-begin-index=${index}"
    args="${args} --cluster-counts=${counts}"
    args="${args} --cluster-with-works=${cluster_with_works}"

    (exec "${REPO_DIR}"/maestroperf spoke $args) &> ${agent_log_dir}/agents-$index.log &
    PERF_PID=$!
    echo "agents started: $PERF_PID"
    touch $pid_dir/$PERF_PID

    agent_total=$(($agent_total + $counts))
    index=$(($index + $counts))
    kind_index=$(($kind_index + 1))
done

