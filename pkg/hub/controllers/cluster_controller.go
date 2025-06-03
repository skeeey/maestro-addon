package controllers

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	kubeapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1"
	clusterlisters "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"github.com/stolostron/maestro-addon/pkg/helpers"
	"github.com/stolostron/maestro-addon/pkg/mq"
)

type ManagedClusterController struct {
	clusterLister            clusterlisters.ManagedClusterLister
	maestroAPIClient         *openapi.APIClient
	messageQueueAuthzCreator mq.MessageQueueAuthzCreator
	rateLimiter              workqueue.TypedRateLimiter[string]
}

func NewManagedClusterController(maestroServiceAddress string,
	clusterInformer clusterinformers.ManagedClusterInformer,
	messageQueueAuthzCreator mq.MessageQueueAuthzCreator,
	recorder events.Recorder) factory.Controller {
	controller := &ManagedClusterController{
		clusterLister:            clusterInformer.Lister(),
		maestroAPIClient:         helpers.NewMaestroAPIClient(maestroServiceAddress),
		messageQueueAuthzCreator: messageQueueAuthzCreator,
		rateLimiter:              workqueue.NewTypedItemExponentialFailureRateLimiter[string](5*time.Second, 300*time.Second),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetName()
		}, clusterInformer.Informer()).
		WithSync(controller.sync).
		ToController("ManagedClusterController", recorder)
}

func (c *ManagedClusterController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	logger := klog.FromContext(ctx)

	clusterName := controllerContext.QueueKey()

	logger.V(4).Info("Reconciling ManagedCluster", "managedClusterName", clusterName)

	managedCluster, err := c.clusterLister.Get(clusterName)
	if kubeapierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if !managedCluster.DeletionTimestamp.IsZero() {
		// TODO delete this cluster in the maestro
		return nil
	}

	if !meta.IsStatusConditionTrue(managedCluster.Status.Conditions, clusterv1.ManagedClusterConditionJoined) {
		// the cluster is not joined yet, do nothing
		return nil
	}

	if err := c.ensureConsumer(ctx, clusterName); err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			logger.V(2).Info(fmt.Sprintf("Requeue the cluster %s to wait the maestro service ready", clusterName))
			controllerContext.Queue().AddAfter(clusterName, c.rateLimiter.When(clusterName))
			return nil
		}

		return err
	}

	return c.ensureACLs(ctx, clusterName)
}

func (c *ManagedClusterController) ensureConsumer(ctx context.Context, managedClusterName string) error {
	existed, err := helpers.FindConsumerByName(ctx, c.maestroAPIClient, managedClusterName)
	if err != nil {
		return err
	}

	if existed {
		return nil
	}

	// create a consumer in the maestro
	return helpers.CreateConsumer(ctx, c.maestroAPIClient, managedClusterName)
}

func (c *ManagedClusterController) ensureACLs(ctx context.Context, managedClusterName string) error {
	if c.messageQueueAuthzCreator != nil {
		return c.messageQueueAuthzCreator.CreateAuthorizations(ctx, managedClusterName)
	}

	return nil
}
