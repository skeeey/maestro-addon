#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

work_dir=${REPO_DIR}/_output/performance/acm/grpc
config_dir=${work_dir}/config
cert_dir=${work_dir}/certs

mkdir -p $config_dir
mkdir -p $cert_dir

echo "$work_dir"

helm install maestro ${REPO_DIR}/charts/maestro-addon
kubectl -n maestro wait deploy/maestro-db --for=condition=Available --timeout=300s
kubectl -n maestro wait deploy/maestro --for=condition=Available --timeout=300s

kubectl -n maestro annotate route maestro-grpc-broker haproxy.router.openshift.io/balance="leastconn"
kubectl -n maestro annotate route maestro-grpc-broker haproxy.router.openshift.io/rate-limit-connections="false"
kubectl -n maestro annotate route maestro-grpc-broker haproxy.router.openshift.io/rate-limit-connections.rate-tcp="30000"
kubectl -n maestro annotate route maestro-grpc-broker haproxy.router.openshift.io/rate-limit-connections.concurrent-tcp="30000"
kubectl -n maestro annotate route maestro-grpc-broker haproxy.router.openshift.io/timeout="3600s"
kubectl -n maestro annotate route maestro-grpc-broker haproxy.router.openshift.io/timeout-tunnel="6h"

host=$(kubectl -n maestro get route maestro-grpc-broker -ojsonpath='{.spec.host}')
kubectl -n open-cluster-management-hub get secrets maestro-signer -ojsonpath="{.data.tls\.crt}" | base64 -d > ${cert_dir}/clients-ca.crt
kubectl -n open-cluster-management-hub get secrets maestro-signer -ojsonpath="{.data.tls\.key}" | base64 -d > ${cert_dir}/clients-ca.key
cp ${cert_dir}/clients-ca.crt ${cert_dir}/cluster-ca.crt

nohup kubectl port-forward svc/maestro 8000 -n maestro > maestro.svc.log 2>&1 &

sleep 30

pushd ${REPO_DIR}/test/performance
go run pkg/hub/maestro/configs/main.go --work-dir=${work_dir} --server=${host} --broker=grpc > configs.log 2>configs.err.log
popd

pushd ${REPO_DIR}/test/performance
go run pkg/hub/maestro/clusters/main.go  > clusters.log 2>clusters.err.log
popd

db_pod_name=$(kubectl -n maestro get pods -l name=maestro-db -ojsonpath='{.items[0].metadata.name}')
kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select count(*) from consumers'

echo "run go run pkg/spoke/main.go --broker=grpc --work-dir=$work_dir --cluster-begin-index=1 > agent-100.log 2>agent-100.err.log &"

echo "run kubectl port-forward svc/maestro-grpc 8090 -n maestro"
echo "run go run pkg/hub/maestro/works/main.go --cluster-begin-index=1"


kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select count(*) from resources'
