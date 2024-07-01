# Performance Test

## Cases

### Maestro Server

#### Case1

1. Create 100 clusters, each cluster without manifestworks
2. Start 100 agents
3. After agents is stated, create M(10) works on each clusters
4. Repeats 90 times

#### Case2

1. Create N clusters, each cluster has M(10) manifestworks
2. Start N agent, agents will resync the works
3. After agents is stated, create M(10) works on each clusters

maestro server cpu/memory
kafka cluster  cpu/memory
postgresql cpu/memory

consumer (cluster) db recorder creation time
kafka topic and role creation time
resource creation time
resource status updated time

### Agent
maestro agent (kafka) compare with work agent (kube) with M(100) works


### Source Client
How many memory/cpu is consumed with NxM works?

## Prepare Test Env

1. An OCP cluster (TODO configurations)
2. An AMQ Streams Operator is installed
3. Deploy a Kafka cluster in `amq-streams` namespace with `kubectl -n amq-streams apply -f test/performance/hack/kafka/kafka-cr.yaml`
4. Deploy a Maestro with `helm install maestro test/performance/hack/charts/maestro --set global.imageOverrides.maestroImage=quay.io/skeeey/maestro:latest`

## Run Test


## Cleanup

```sh
helm uninstall maestro
```