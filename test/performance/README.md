# cases

1. Create N clusters, each cluster without manifestworks
2. Start N agents
3. After agents is stated, create works


1. Create N clusters, each cluster has M manifestworks
2. Start N agent, agents will resync the works
3. After agents is stated, create works

maestro server cpu/memory
kafka cluster  cpu/memory
postgresql cpu/memory

consumer (cluster) db recorder creation time
kafka topic and role creation time
resource creation time
resource status updated time

maestro agent (kafka) compare with work agent (kube)


TODO source work client
- watch
- informer
- how many memory/cpu is needed?

## Deploy Maestro

```sh
helm install maestro test/performance/hack/charts/maestro \
  --set global.imageOverrides.maestroImage=quay.io/skeeey/maestro:latest \
  --set messageQueue.amqStreams.namespace=strimzi \
  --set messageQueue.amqStreams.listener.type=internal \
  --set messageQueue.amqStreams.listener.port=9093
```

```sh
helm uninstall maestro
```