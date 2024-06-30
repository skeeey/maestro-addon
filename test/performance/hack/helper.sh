#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

function apply_job() {
  kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: clusters-$2-$3
  namespace: maestro
spec:
  template:
    spec:
      containers:
      - name: topics
        image: quay.io/skeeey/maestro-perf-tool
        imagePullPolicy: Always
        args:
          - "/maestroperf"
          - "prepare"
          - "--cluster-begin-index=$2"
          - "--cluster-counts=$3"
          - "--only-works=$1"
        volumeMounts:
        - mountPath: "/configs/kafka"
          name: maestro-kafka-config
        - mountPath: "/secrets/certs/kafka"
          name: kafka-client-certs
      restartPolicy: Never
      volumes:
      - name: maestro-kafka-config
        secret:
          secretName: maestro-kafka-config
      - name: kafka-client-certs
        secret:
          secretName: kafka-client-certs
  backoffLimit: 4
EOF
}

# start helper command

if [ "$1"x = "clusters"x ]; then
  echo "create $3 clusters from index $2"
  apply_job "false" $2 $3
  exit
fi

if [ "$1"x = "clusters-with-works"x ]; then
  echo "create $3 clusters with works from index $2"
  apply_job "false" $2 $3
  exit
fi

if [ "$1"x = "works"x ]; then
  echo "create works from maestro-cluster-$2 to maestro-cluster-$[$2+$3 - 1]"
  apply_job "true" $2 $3
  exit
fi

echo "Unsupported command: $1"
exit 1
