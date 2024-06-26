package controllers

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/maestro-addon/pkg/helpers"
	"github.com/stolostron/maestro-addon/pkg/mq"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1"
	clusterlisters "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

type ManagedClusterController struct {
	clusterLister            clusterlisters.ManagedClusterLister
	maestroAPIClient         *openapi.APIClient
	messageQueueAuthzCreator mq.MessageQueueAuthzCreator
}

func NewManagedClusterController(maestroServiceAddress string,
	clusterInformer clusterinformers.ManagedClusterInformer,
	messageQueueAuthzCreator mq.MessageQueueAuthzCreator,
	recorder events.Recorder) factory.Controller {
	controller := &ManagedClusterController{
		clusterLister:            clusterInformer.Lister(),
		maestroAPIClient:         helpers.NewMaestroAPIClient(maestroServiceAddress),
		messageQueueAuthzCreator: messageQueueAuthzCreator,
	}

	return factory.New().
		WithInformersQueueKeysFunc(func(obj runtime.Object) []string {
			accessor, _ := meta.Accessor(obj)
			return []string{accessor.GetName()}
		}, clusterInformer.Informer()).
		WithSync(controller.sync).
		ToController("ManagedClusterController", recorder)
}

func (c *ManagedClusterController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	logger := klog.FromContext(ctx)

	managedClusterName := controllerContext.QueueKey()

	logger.V(4).Info("Reconciling ManagedCluster", "managedClusterName", managedClusterName)

	managedCluster, err := c.clusterLister.Get(managedClusterName)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if !managedCluster.DeletionTimestamp.IsZero() {
		// TODO delete this cluster in the maestro
		return nil
	}

	if meta.IsStatusConditionFalse(managedCluster.Status.Conditions, clusterv1.ManagedClusterConditionJoined) {
		// the cluster is not joined yet, do nothing
		return nil
	}

	if err := c.ensureConsumer(ctx, managedClusterName); err != nil {
		return err
	}

	if err := c.ensureACLs(ctx, managedClusterName); err != nil {
		return err
	}

	return nil
}

func (c *ManagedClusterController) ensureConsumer(ctx context.Context, managedClusterName string) error {
	existed, err := c.findConsumerByName(ctx, managedClusterName)
	if err != nil {
		return err
	}

	if existed {
		return nil
	}

	// create a consumer in the maestro
	_, _, err = c.maestroAPIClient.DefaultApi.ApiMaestroV1ConsumersPost(ctx).
		Consumer(openapi.Consumer{Name: openapi.PtrString(managedClusterName)}).
		Execute()
	return err
}

func (c *ManagedClusterController) ensureACLs(ctx context.Context, managedClusterName string) error {
	if c.messageQueueAuthzCreator != nil {
		return c.messageQueueAuthzCreator.CreateAuthorizations(ctx, managedClusterName)
	}

	return nil
}

func (c *ManagedClusterController) findConsumerByName(ctx context.Context, managedClusterName string) (bool, error) {
	list, _, err := c.maestroAPIClient.DefaultApi.ApiMaestroV1ConsumersGet(ctx).
		Search(fmt.Sprintf("name = '%s'", managedClusterName)).
		Execute()
	if err != nil {
		return false, err
	}

	for _, consumer := range list.Items {
		if consumer.Name == &managedClusterName {
			return true, nil
		}
	}

	return false, nil
}
