#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

total=${total:-100}
begin_index=${begin_index:-1}

lastIndex=$(($begin_index + $total - 1))
echo "create works from maestro-cluster-$begin_index to maestro-cluster-$lastIndex"

kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: works-$begin_index-$lastIndex
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
          - "--cluster-begin-index=$begin_index"
          - "--cluster-counts=$total"
          - "--only-works=true"
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
