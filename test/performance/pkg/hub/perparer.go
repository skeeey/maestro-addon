package hub

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/source/codec"

	"github.com/stolostron/maestro-addon/pkg/helpers"
	"github.com/stolostron/maestro-addon/pkg/mq"
	"github.com/stolostron/maestro-addon/test/performance/pkg/hub/store"
	"github.com/stolostron/maestro-addon/test/performance/pkg/hub/workloads"
	"github.com/stolostron/maestro-addon/test/performance/pkg/util"
)

const (
	sourceID                     = "maestro-performance-test"
	defaultMaestroServiceAddress = "http://maestro:8000"
	defaultMaestroGRPCAddress    = "http://maestro:8090"
)

type PreparerOptions struct {
	MaestroServiceAddress        string
	GRPCServiceAddress           string
	MessageQueueBrokerConfigPath string

	ClusterBeginIndex int
	ClusterCounts     int
	ClusterWithWorks  bool

	OnlyWorks bool

	OnlyTopics bool
}

func NewPreparerOptions() *PreparerOptions {
	return &PreparerOptions{
		MaestroServiceAddress:        defaultMaestroServiceAddress,
		GRPCServiceAddress:           defaultMaestroGRPCAddress,
		MessageQueueBrokerConfigPath: "/configs/kafka/config.yaml",
		ClusterBeginIndex:            0,
		ClusterCounts:                1,
	}
}

func (o *PreparerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MaestroServiceAddress, "maestro-service-address", o.MaestroServiceAddress, "Address of the Maestro API service")
	fs.StringVar(&o.GRPCServiceAddress, "grpc-service-address", o.GRPCServiceAddress, "Address of the Maestro GRPC service")
	fs.StringVar(&o.MessageQueueBrokerConfigPath, "message-queue-broker-config", o.MessageQueueBrokerConfigPath, "Path to the message queue broker configuration file")
	fs.IntVar(&o.ClusterBeginIndex, "cluster-begin-index", o.ClusterBeginIndex, "Begin index of the clusters")
	fs.IntVar(&o.ClusterCounts, "cluster-counts", o.ClusterCounts, "Counts of the clusters")
	fs.BoolVar(&o.ClusterWithWorks, "cluster-with-works", o.ClusterWithWorks, "Create cluster with initialized works")
	fs.BoolVar(&o.OnlyWorks, "only-works", o.OnlyWorks, "Only create works for given clusters")
	fs.BoolVar(&o.OnlyTopics, "only-topics", o.OnlyTopics, "Only initialize kafka topics")
}

func (o *PreparerOptions) Run(ctx context.Context) error {
	if o.OnlyTopics {
		return o.PrepareTopics()
	}

	if o.OnlyWorks {
		return o.CreateWorks(ctx, "work")
	}

	if err := o.PrepareClusters(ctx); err != nil {
		return err
	}

	if !o.ClusterWithWorks {
		return nil
	}

	// initialize cluster with works
	if err := o.CreateWorks(ctx, "init"); err != nil {
		return err
	}

	return nil
}

func (o *PreparerOptions) PrepareTopics() error {
	_, err := mq.NewMessageQueueAuthzCreator(mq.MessageQueueKafka, o.MessageQueueBrokerConfigPath)
	return err
}

func (o *PreparerOptions) PrepareClusters(ctx context.Context) error {
	apiClient := helpers.NewMaestroAPIClient(o.MaestroServiceAddress)

	mqAuthzCreator, err := mq.NewMessageQueueAuthzCreator(mq.MessageQueueKafka, o.MessageQueueBrokerConfigPath)
	if err != nil {
		return err
	}

	index := o.ClusterBeginIndex
	startTime := time.Now()
	for i := 0; i < o.ClusterCounts; i++ {
		index = index + 1
		clusterName := util.ClusterName(index)

		startTime := time.Now()
		if err := helpers.CreateConsumer(ctx, apiClient, clusterName); err != nil {
			return err
		}

		klog.V(4).Infof("cluster %s is created, time=%dms", clusterName, util.UsedTime(startTime, time.Millisecond))

		startTime = time.Now()
		if err := mqAuthzCreator.CreateAuthorizations(ctx, clusterName); err != nil {
			return err
		}
		klog.V(4).Infof("the kafka auth is prepared for cluster %s, time=%dms",
			clusterName, util.UsedTime(startTime, time.Millisecond))
	}
	klog.Infof("Clusters (%d) are created, time=%sms", o.ClusterCounts, util.UsedTime(startTime, time.Millisecond))
	return nil
}

func (o *PreparerOptions) CreateWorks(ctx context.Context, workType string) error {
	creator, err := work.NewClientHolderBuilder(grpc.GRPCOptions{URL: o.GRPCServiceAddress}).
		WithClientID(fmt.Sprintf("%s-client", sourceID)).
		WithSourceID(sourceID).
		WithCodecs(codec.NewManifestBundleCodec()).
		WithWorkClientWatcherStore(store.NewCreateOnlyWatcherStore()).
		NewSourceClientHolder(ctx)
	if err != nil {
		return err
	}

	workClient := creator.WorkInterface()

	index := o.ClusterBeginIndex
	total := 0
	startTime := time.Now()
	for i := 0; i < o.ClusterCounts; i++ {
		index = index + 1
		clusterName := util.ClusterName(index)
		works, err := workloads.ToManifestWorks(clusterName, workType)
		if err != nil {
			return err
		}

		startTime := time.Now()
		for _, work := range works {
			startTime := time.Now()
			if _, err := workClient.WorkV1().ManifestWorks(clusterName).Create(
				ctx,
				work,
				metav1.CreateOptions{},
			); err != nil {
				return err
			}

			klog.V(4).Infof("the work %s/%s is created, time=%dms",
				work.Namespace, work.Name, util.UsedTime(startTime, time.Millisecond))
			total = total + 1
		}

		klog.V(4).Infof("the works are created for cluster %s, time=%dms",
			clusterName, util.UsedTime(startTime, time.Millisecond))
	}

	klog.Infof("Works (%d) are created, time=%dms", total, util.UsedTime(startTime, time.Millisecond))

	return nil
}
