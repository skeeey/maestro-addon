#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

work_dir=${REPO_DIR}/_output/performance/acm
config_dir=${work_dir}/config
cert_dir=${work_dir}/certs

mkdir -p $config_dir
mkdir -p $cert_dir

kubectl create ns amq-streams
kubectl -n amq-streams apply -f ${REPO_DIR}/test/performance/hack/kafka/kafka-cr.yaml
kubectl -n amq-streams wait kafka/kafka --for=condition=Ready --timeout=600s

kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/disable_cookies="true"
kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/balance="leastconn"
kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/rate-limit-connections="false"
# Note: one agent needs about 5 connections
kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/rate-limit-connections.rate-tcp="30000"
kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/rate-limit-connections.concurrent-tcp="30000"
kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/timeout="3600s"
kubectl -n amq-streams annotate route kafka-kafka-tls-bootstrap haproxy.router.openshift.io/timeout-tunnel="6h"

sleep 60

helm install maestro ${REPO_DIR}/charts/maestro-addon --set maestro.broker="kafka"
kubectl -n maestro wait deploy/maestro-db --for=condition=Available --timeout=300s
kubectl -n maestro wait deploy/maestro --for=condition=Available --timeout=300s

sleep 10

kafka_host=$(kubectl -n amq-streams get route kafka-kafka-tls-bootstrap -ojsonpath='{.spec.host}')
kafka_host="${kafka_host}:443"

kubectl -n amq-streams get secrets kafka-cluster-ca-cert -ojsonpath="{.data.ca\.crt}" | base64 -d > ${cert_dir}/cluster-ca.crt
kubectl -n amq-streams get secrets kafka-clients-ca-cert -ojsonpath="{.data.ca\.crt}" | base64 -d > ${cert_dir}/clients-ca.crt
kubectl -n amq-streams get secrets kafka-clients-ca -ojsonpath="{.data.ca\.key}" | base64 -d > ${cert_dir}/clients-ca.key

kubectl -n maestro get secrets kafka-client-certs -ojsonpath="{.data.client\.crt}" | base64 -d > ${cert_dir}/admin-client.crt
kubectl -n maestro get secrets kafka-client-certs -ojsonpath="{.data.client\.key}" | base64 -d > ${cert_dir}/admin-client.key

rm -f $config_dir/kafka.admin.config
echo "bootstrapServer: $kafka_host" > $config_dir/kafka.admin.config
echo "caFile: $cert_dir/cluster-ca.crt" >> $config_dir/kafka.admin.config
echo "clientCertFile: $cert_dir/admin-client.crt" >> $config_dir/kafka.admin.config
echo "clientKeyFile: $cert_dir/admin-client.key" >> $config_dir/kafka.admin.config

echo "$kafka_host"
echo "$work_dir"

pushd ${REPO_DIR}/test/performance
go run pkg/hub/maestro/configs/main.go --work-dir=${work_dir} --server=${kafka_host} --broker=kafka > configs.log 2>configs.err.log
popd

nohup kubectl port-forward svc/maestro 8000 -n maestro > maestro.svc.log 2>&1 &

sleep 10

pushd ${REPO_DIR}/test/performance
go run pkg/hub/maestro/clusters/main.go  > clusters.log 2>clusters.err.log
popd

db_pod_name=$(kubectl -n maestro get pods -l name=maestro-db -ojsonpath='{.items[0].metadata.name}')
kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select count(*) from consumers'

echo "run go run -tags=kafka pkg/spoke/main.go --broker=kafka --work-dir=$work_dir --cluster-begin-index=1 > agent-100.log 2>agent-100.err.log &"

echo "run kubectl port-forward svc/maestro-grpc 8090 -n maestro"
echo "run go run pkg/hub/maestro/works/main.go --cluster-begin-index=1"
