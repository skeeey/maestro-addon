#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

kubectl delete namespaces -l maestro.performance.test=acm --ignore-not-found
kubectl delete namespaces amq-streams --ignore-not-found

helm uninstall maestro --ignore-not-found

rm -rf ${REPO_DIR}/_output/performance/acm
