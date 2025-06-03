package hub

import (
	"context"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/pflag"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"

	"github.com/stolostron/maestro-addon/pkg/common"
	"github.com/stolostron/maestro-addon/pkg/hub/controllers"
	"github.com/stolostron/maestro-addon/pkg/mq"
)

const defaultMaestroServiceAddress = "http://maestro:8000"

type MaestroAddOnManagerOptions struct {
	messageQueueBrokerType       string
	messageQueueBrokerConfigPath string
	maestroServiceAddress        string
}

func NewMaestroAddOnManagerOptions() *MaestroAddOnManagerOptions {
	return &MaestroAddOnManagerOptions{
		maestroServiceAddress:        defaultMaestroServiceAddress,
		messageQueueBrokerType:       common.BrokerKafka,
		messageQueueBrokerConfigPath: "/configs/kafka/config.yaml",
	}
}

func (o *MaestroAddOnManagerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.maestroServiceAddress, "maestro-service-address", o.maestroServiceAddress,
		"Address of the Maestro API service")
	fs.StringVar(&o.messageQueueBrokerType, "message-queue-broker-type", o.messageQueueBrokerType,
		"Type of message queue broker")
	fs.StringVar(&o.messageQueueBrokerConfigPath, "message-queue-broker-config", o.messageQueueBrokerConfigPath,
		"Path to the message queue broker configuration file")
}

func (o *MaestroAddOnManagerOptions) RunHubManager(
	ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	clusterClient, err := clusterclientset.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterInformers := clusterinformers.NewSharedInformerFactory(clusterClient, 30*time.Minute)

	mqAuthzCreator, err := mq.NewMessageQueueAuthzCreator(o.messageQueueBrokerType, o.messageQueueBrokerConfigPath)
	if err != nil {
		return err
	}

	managedClusterController := controllers.NewManagedClusterController(
		o.maestroServiceAddress,
		clusterInformers.Cluster().V1().ManagedClusters(),
		mqAuthzCreator,
		controllerContext.EventRecorder,
	)

	go clusterInformers.Start(ctx.Done())

	go managedClusterController.Run(ctx, 1)

	<-ctx.Done()
	return nil
}
