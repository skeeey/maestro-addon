package mock

import (
	"testing"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/events/eventstesting"
	"k8s.io/client-go/util/workqueue"
)

type MockSyncContext struct {
	key      string
	recorder events.Recorder
	queue    workqueue.RateLimitingInterface // nolint:staticcheck
}

var _ factory.SyncContext = &MockSyncContext{}

// nolint:staticcheck
func (m MockSyncContext) Queue() workqueue.RateLimitingInterface { return m.queue }
func (m MockSyncContext) QueueKey() string                       { return m.key }
func (m MockSyncContext) Recorder() events.Recorder              { return m.recorder }

func NewMockSyncContext(t *testing.T, key string) *MockSyncContext {
	return &MockSyncContext{
		key:      key,
		recorder: eventstesting.NewTestingEventRecorder(t),
		// nolint:staticcheck
		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
}
