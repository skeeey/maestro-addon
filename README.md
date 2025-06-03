# maestro-addon

The maestro addon is used to enable the maestro in the ACM

## Build

### Build binary

```
make build
```

### Build Image

```
make image
```

## Deploy

### Install maestro-addon on ACM hub

Run following command to install maestro-addon on ACM hub

```sh
helm install maestro-addon ./charts/maestro-addon
```

- To use customized images:

  ```
  --set global.imageOverrides.maestroImage=<your customized image>
  --set global.imageOverrides.maestroAddOnImage=<your customized image>
  ```

- (Optional) To use AMQ Streams Kafka as the events broker

  1. Follow this [doc](https://access.redhat.com/documentation/en-us/red_hat_amq_streams/2.6/html/deploying_and_managing_amq_streams_on_openshift/operator-hub-str#proc-deploying-cluster-operator-hub-str) to install the AMQ Streams Operator from OpenShift OperatorHub
  2. After the operator is installed, create a Kafka CR in one namespace to provision a Kafka cluster, the Kafka CR must enable the authorization with a super user `CN=maestro-kafka-admin` and a route listener with mTLS, There is an [example](contrib/examples/kafka-cr.yaml) for Kafka CR.
  3. Specify the AMQ Streams Kafka cluster name and namespace:

      ```
      --set maestro.broker=kafka
      --set messageQueue.amqStreams.name=<your AMQ Streams Kafka CR name, by default is kafka>
      --set messageQueue.amqStreams.namespace=<your AMQ Streams Kafka CR namespace, by default is amq-streams>
      ```

More available config values can be found from [here](charts/maestro-addon/values.yaml).

Using `helm uninstall maestro-addon` to uninstall the maestro-addon.

### Install maestro-addon agent on a managed cluster

Create a ManagedClusterAddOn CR in one managed cluster namespace on an ACM hub to install the maestro-addon agent on that managed cluster.

```sh
cat << EOF | oc -n <cluster-name> apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: maestro-addon
spec:
  installNamespace: open-cluster-management-agent
EOF
```

Using `oc -n <cluster-name>  delete ManagedClusterAddOn maestro-addon` to uninstall the maestro-addon agent from a managed cluster.

## Using ManifestWorkReplicaSet

### Enable the ManifestWorkReplicaSet controller with cloudevents on the hub

1. Prepare the config for ManifestWorkReplicaSet controller

```sh
cat << EOF | oc -n multicluster-engine create -f -
apiVersion: v1
kind: Secret
metadata:
  name: work-driver-config
type: Opaque
stringData:
  config.yaml: |
    url: maestro-grpc.maestro:8090
EOF
```

2. Enable the ManifestWorkReplicaSet controller

```sh
oc patch clustermanager cluster-manager --type=merge -p='{"spec":{"workConfiguration":{"workDriver":"grpc","featureGates":[{"feature":"CloudEventsDrivers","mode":"Enable"},{"feature":"ManifestWorkReplicaSet","mode":"Enable"}]}}}'
```

Using the following command to disable the ManifestWorkReplicaSet controller:

```sh
oc patch clustermanager cluster-manager --type=merge -p='{"spec":{"workConfiguration":{"featureGates":[{"feature":"CloudEventsDrivers","mode":"Disable"},{"feature":"ManifestWorkReplicaSet","mode":"Disable"}]}}}'
```

### Use ManifestWorkReplicaSet to deploy kube resources on managed clusters

Using the following example to deploy a `busybox` in the namespace `mwrs-test` on your managed clusters

1. Prepare the Placement for ManifestWorkReplicaSet in the hub `default` namespace

```sh
oc apply -f contrib/examples/placement
```

2. Apply the ManifestWorkReplicaSet CR example in the hub `default` namespace

```sh
oc apply -f contrib/examples/manifestworkreplicaset.yaml
```

3. Check the status of the ManifestWorkReplicaSet

```sh
oc -n default get manifestworkreplicasets busybox

NAME      PLACEMENT    FOUND   MANIFESTWORKS   APPLIED
busybox   AsExpected   True    AsExpected      True
```

Using `oc delete -f contrib/examples/manifestworkreplicaset.yaml` to delete the ManifestWorkReplicaSet
