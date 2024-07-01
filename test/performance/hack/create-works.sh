#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

function apply_job() {
  kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: $1-$4-$5
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
          - "--cluster-begin-index=$4"
          - "--cluster-counts=$5"
          - "--cluster-with-works=$3"
          - "--only-works=$2"
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

if [ "$1"x = "works"x ]; then
  echo "create works from maestro-cluster-$2 to maestro-cluster-$[$2+$3 - 1]"
  apply_job "works" "true" "false" $2 $3
  exit
fi

echo "Unsupported command: $1"
exit 1
