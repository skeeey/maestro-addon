package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/options"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/agent/codec"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/store"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/kafka"

	"github.com/stolostron/maestro-addon/test/performance/pkg/common"
	"github.com/stolostron/maestro-addon/test/performance/pkg/util"
)

const ns = "playback-ns"

const (
	nsStatus     = `{"phase":"Active"}`
	svcStatus    = `{"phase":"Active"}`
	deployStatus = `
{
	"availableReplicas":3,
	"conditions":[
		{
			"lastTransitionTime":"2024-09-29T01:47:46Z",
			"lastUpdateTime":"2024-09-29T01:47:46Z",
			"message":"Deployment has minimum availability.",
			"reason":"MinimumReplicasAvailable",
			"status":"True","type":"Available"
		},{
			"lastTransitionTime":"2024-09-29T01:33:58Z",
			"lastUpdateTime":"2024-09-29T01:47:46Z",
			"message":"ReplicaSet \"frontend-76fb487c8f\" has successfully progressed.",
			"reason":"NewReplicaSetAvailable",
			"status":"True",
			"type":"Progressing"
		}],
	"observedGeneration":4,
	"readyReplicas":3,
	"replicas":3,
	"updatedReplicas":3
}`
)

var (
	workDir           = flag.String("work-dir", "", "")
	broker            = flag.String("broker", "", "")
	clusterBeginIndex = flag.Int("cluster-begin-index", 1, "Begin index of the clusters")
	clusterCounts     = flag.Int("cluster-counts", common.DEFAULT_AGENT_COUNTS, "Counts of the clusters")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer cancel()
		<-stopCh
	}()

	fmt.Printf("==== %s\n", time.Now().Format("2006-01-02 15:04:05"))
	index := *clusterBeginIndex
	for i := 0; i < *clusterCounts; i++ {
		clusterName := util.ClusterName(index)
		go func(clusterName string) {
			if err := startWorkAgent(ctx, clusterName); err != nil {
				log.Fatal(err)
			}
		}(clusterName)
		fmt.Printf("agent %s is started\n", clusterName)
		if i > 99 && i%100 == 0 {
			time.Sleep(10 * time.Second)
		}
		index++
	}

	<-ctx.Done()
}

func startWorkAgent(ctx context.Context, clusterName string) error {
	updatedWorks := 0
	watcherStore := store.NewAgentInformerWatcherStore()

	var config any
	var err error
	switch *broker {
	case "kafka":
		config, err = kafka.BuildKafkaOptionsFromFlags(
			filepath.Join(*workDir, "config", fmt.Sprintf("client-%s.config", clusterName)))
		if err != nil {
			return err
		}
	case "grpc":
		config, err = grpc.BuildGRPCOptionsFromFlags(
			filepath.Join(*workDir, "config", fmt.Sprintf("client-%s.config", clusterName)))
		if err != nil {
			return err
		}
	}

	clientID := clusterName + "-" + rand.String(5)
	opt := options.NewGenericClientOptions(config, codec.NewManifestBundleCodec(), clientID).
		WithClientWatcherStore(watcherStore).
		WithResyncEnabled(false).
		WithClusterName(clusterName)
	clientHolder, err := work.NewAgentClientHolder(ctx, opt)
	if err != nil {
		return err
	}

	informerCtx, informerCancel := context.WithCancel(ctx)
	factory := workinformers.NewSharedInformerFactoryWithOptions(
		clientHolder.WorkInterface(),
		24*time.Hour,
		workinformers.WithNamespace(clusterName),
	)
	informer := factory.Work().V1().ManifestWorks().Informer()
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			work, ok := obj.(*workv1.ManifestWork)
			if !ok {
				fmt.Printf("unknown type %v\n", obj)
			}

			patches, err := patch(work)
			if err != nil {
				fmt.Printf("error: %v\n", err)
			}

			if _, err := clientHolder.ManifestWorks(clusterName).Patch(
				ctx,
				work.Name,
				types.MergePatchType,
				patches,
				metav1.PatchOptions{},
				"status",
			); err != nil {
				fmt.Printf("error: %v\n", err)
			}

			updatedWorks++
			if updatedWorks == 50 {
				fmt.Printf("works %d in %s are updated\n", updatedWorks, clusterName)
				informerCancel()
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {},
		DeleteFunc: func(obj interface{}) {},
	}); err != nil {
		log.Fatal(err)
	}
	watcherStore.SetInformer(informer)

	go informer.Run(informerCtx.Done())

	return nil
}

func patch(work *workv1.ManifestWork) ([]byte, error) {
	oldData, err := json.Marshal(work)
	if err != nil {
		return nil, err
	}

	newWork := work.DeepCopy()
	lastTransitionTime := metav1.Now()
	newWork.Status = workv1.ManifestWorkStatus{
		Conditions: workConditions(lastTransitionTime),
		ResourceStatus: workv1.ManifestResourceStatus{
			Manifests: []workv1.ManifestCondition{
				nsManifestCondition(0, lastTransitionTime),
				deployManifestCondition(1, "frontend", lastTransitionTime),
				svcManifestCondition(2, "frontend", lastTransitionTime),
				deployManifestCondition(3, "redis-master", lastTransitionTime),
				svcManifestCondition(4, "redis-master", lastTransitionTime),
				deployManifestCondition(5, "redis-slave", lastTransitionTime),
				svcManifestCondition(6, "redis-slave", lastTransitionTime),
			},
		},
	}

	newData, err := json.Marshal(newWork)
	if err != nil {
		return nil, err
	}

	return jsonpatch.CreateMergePatch(oldData, newData)
}

func workConditions(lastTransitionTime metav1.Time) []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               "Applied",
			Status:             metav1.ConditionTrue,
			Reason:             "AppliedManifestWorkComplete",
			Message:            "Apply manifest work complete",
			ObservedGeneration: 3,
			LastTransitionTime: lastTransitionTime,
		},
		{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			Reason:             "ResourceAvailable",
			Message:            "All resources are available",
			ObservedGeneration: 3,
			LastTransitionTime: lastTransitionTime,
		},
	}
}

func resourceConditions(lastTransitionTime metav1.Time) []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               "Applied",
			Status:             metav1.ConditionTrue,
			Reason:             "AppliedManifestWorkComplete",
			Message:            "Apply manifest work complete",
			LastTransitionTime: lastTransitionTime,
		},
		{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			Reason:             "ResourceAvailable",
			Message:            "Resources is available",
			LastTransitionTime: lastTransitionTime,
		},
		{
			Type:               "StatusFeedbackSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "StatusFeedbackSynced",
			LastTransitionTime: lastTransitionTime,
		},
	}
}

func nsManifestCondition(ordinal int32, lastTransitionTime metav1.Time) workv1.ManifestCondition {
	return workv1.ManifestCondition{
		Conditions: resourceConditions(lastTransitionTime),
		ResourceMeta: workv1.ManifestResourceMeta{
			Resource: "namespaces",
			Version:  "v1",
			Kind:     "Namespace",
			Ordinal:  ordinal,
			Name:     ns,
		},
		StatusFeedbacks: workv1.StatusFeedbackResult{
			Values: []workv1.FeedbackValue{
				{
					Name: "status",
					Value: workv1.FieldValue{
						Type:    workv1.JsonRaw,
						JsonRaw: ptr.To(nsStatus),
					},
				},
			},
		},
	}
}

func svcManifestCondition(ordinal int32, name string, lastTransitionTime metav1.Time) workv1.ManifestCondition {
	return workv1.ManifestCondition{
		Conditions: resourceConditions(lastTransitionTime),
		ResourceMeta: workv1.ManifestResourceMeta{
			Resource:  "services",
			Version:   "v1",
			Kind:      "Service",
			Ordinal:   ordinal,
			Name:      name,
			Namespace: ns,
		},
		StatusFeedbacks: workv1.StatusFeedbackResult{
			Values: []workv1.FeedbackValue{
				{
					Name: "status",
					Value: workv1.FieldValue{
						Type:    workv1.JsonRaw,
						JsonRaw: ptr.To(svcStatus),
					},
				},
			},
		},
	}
}

func deployManifestCondition(ordinal int32, name string, lastTransitionTime metav1.Time) workv1.ManifestCondition {
	return workv1.ManifestCondition{
		Conditions: resourceConditions(lastTransitionTime),
		ResourceMeta: workv1.ManifestResourceMeta{
			Group:     "apps",
			Resource:  "deployments",
			Version:   "v1",
			Kind:      "Deployment",
			Ordinal:   ordinal,
			Name:      name,
			Namespace: ns,
		},
		StatusFeedbacks: workv1.StatusFeedbackResult{
			Values: []workv1.FeedbackValue{
				{
					Name: "status",
					Value: workv1.FieldValue{
						Type:    workv1.JsonRaw,
						JsonRaw: ptr.To(deployStatus),
					},
				},
			},
		},
	}
}
