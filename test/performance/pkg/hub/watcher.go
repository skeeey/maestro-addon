package hub

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"github.com/stolostron/maestro-addon/test/performance/pkg/hub/workloads"
	"github.com/stolostron/maestro-addon/test/performance/pkg/util"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	workv1 "open-cluster-management.io/api/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"
	workstore "open-cluster-management.io/sdk-go/pkg/cloudevents/work/store"
)

type WatcherOptions struct {
	GRPCServiceAddress string
	ClusterCounts      int
}

func NewWatcherOptions() *WatcherOptions {
	return &WatcherOptions{
		GRPCServiceAddress: defaultMaestroGRPCAddress,
		ClusterCounts:      1,
	}
}

func (o *WatcherOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.GRPCServiceAddress, "grpc-service-address", o.GRPCServiceAddress, "Address of the Maestro GRPC service")
	fs.IntVar(&o.ClusterCounts, "cluster-counts", o.ClusterCounts, "Counts of the clusters")
}

func (o *WatcherOptions) Run(ctx context.Context) error {
	watcherStore := workstore.NewAgentInformerWatcherStore()

	creator, err := work.NewClientHolderBuilder(grpc.GRPCOptions{URL: o.GRPCServiceAddress}).
		WithClientID(fmt.Sprintf("%s-client", sourceID)).
		WithSourceID(sourceID).
		WithCodecs(codec.NewManifestBundleCodec()).
		WithWorkClientWatcherStore(watcherStore).
		NewSourceClientHolder(ctx)
	if err != nil {
		return err
	}

	factory := workinformers.NewSharedInformerFactoryWithOptions(creator.WorkInterface(), 5*time.Minute)
	workInformer := factory.Work().V1().ManifestWorks()

	// init store
	informerStore := workInformer.Informer().GetStore()
	works, err := o.GetWorks()
	if err != nil {
		return err
	}
	for _, work := range works {
		informerStore.Add(work)
	}

	watcherStore.SetStore(informerStore)

	go workInformer.Informer().Run(ctx.Done())

	workInformer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			work, ok := obj.(*workv1.ManifestWork)
			if !ok {
				klog.Errorf("error to get object: %v", obj)
				return
			}

			klog.Infof("work %s/%s is added", work.Namespace, work.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newWork, ok := newObj.(*workv1.ManifestWork)
			if !ok {
				klog.Errorf("error to get object: %v", newObj)
				return
			}

			klog.Infof("work %s/%s is updated", newWork.Namespace, newWork.Name)
		},
		DeleteFunc: func(obj interface{}) {},
	})

	return nil
}

func (o *WatcherOptions) GetWorks() ([]*workv1.ManifestWork, error) {
	all := []*workv1.ManifestWork{}

	for i := 0; i < o.ClusterCounts; i++ {
		clusterName := util.ClusterName(i)
		works, err := workloads.ToManifestWorks(clusterName, "init")
		if err != nil {
			return nil, err
		}

		all = append(all, works...)

		works, err = workloads.ToManifestWorks(clusterName, "work")
		if err != nil {
			return nil, err
		}

		all = append(all, works...)
	}

	return all, nil
}
