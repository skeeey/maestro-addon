# Performance Test

## Cases

### Maestro Server

#### Case1

1. Create 100 clusters, each cluster without manifestworks
2. Start 100 agents
3. After agents is stated, create 10/20/50/100 works on each clusters
4. Repeats 90 times

#### Case2

1. Create N clusters, each cluster has M(10) manifestworks
2. Start N agent, agents will resync the works
3. After agents is stated, create 10/20/50/100 works on each clusters

maestro server cpu/memory
kafka cluster  cpu/memory
postgresql cpu/memory

consumer (cluster) db recorder creation time
kafka topic and role creation time
resource creation time
resource status updated time

### Agent
maestro agent (kafka) compare with work agent (kube) with 100 works


### Source Client
How many memory/cpu is consumed with NxM works?

## Prepare Test Env

1. An OCP cluster (TODO configurations)
2. An AMQ Streams Operator is installed on the OCP cluster
3. Create a namespace `amq-streams` in the OCP cluster
4. Deploy a Kafka cluster in `amq-streams` namespace with `kubectl -n amq-streams apply -f test/performance/hack/kafka/kafka-cr.yaml`
5. Deploy a Maestro with `helm install maestro test/performance/hack/charts/maestro --set global.imageOverrides.maestroImage=quay.io/skeeey/maestro:latest`

## Run

```
begin_index=1 total=10 test/performance/hack/start-agents.sh
begin_index=1 total=10 test/performance/hack/create-works.sh
```

### find logs
```
ll _output/performance/logs
```


## Cleanup

```sh
ls _output/performance/pids | xargs kill
kind delete clusters --all
kubectl -n amq-streams delete -f test/performance/hack/kafka/kafka-cr.yaml
kubectl delete ns amq-streams
helm uninstall maestro
rm -rf _output/performance
```