#!/usr/bin/env bash

REPO_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})/../../.." ; pwd -P)"

if [ "$1"x = "topics"x ]; then
    kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: topics
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
          - "--only-topics"
          - "--v=4"
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

    exit $?
fi

if [ "$1"x = "clusters"x ]; then
    exit
fi

if [ "$1"x = "clusters-with-works"x ]; then
    exit
fi

if [ "$1"x = "works"x ]; then
    exit
fi

echo "Unknown command \"$1\""
exit 1
