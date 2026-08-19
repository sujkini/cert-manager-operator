package deployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/clock"

	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

// countActions returns the number of actions recorded on the fake client that
// match the given verb and resource.
func countActions(actions []clienttesting.Action, verb, resource string) int {
	count := 0
	for _, a := range actions {
		if a.GetVerb() == verb && a.GetResource().Resource == resource {
			count++
		}
	}
	return count
}

func newTestUserDefinedController(objects ...*networkingv1.NetworkPolicy) (*CertManagerNetworkPolicyUserDefinedController, *fake.Clientset, events.InMemoryRecorder) {
	initial := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		initial = append(initial, o)
	}
	kubeClient := fake.NewSimpleClientset(initial...)
	recorder := events.NewInMemoryRecorder("test", clock.RealClock{})
	c := &CertManagerNetworkPolicyUserDefinedController{
		kubeClient:    kubeClient,
		eventRecorder: recorder,
	}
	return c, kubeClient, recorder
}

func desiredPolicy() *networkingv1.NetworkPolicy {
	c := &CertManagerNetworkPolicyUserDefinedController{}
	return c.createUserNetworkPolicy(v1alpha1.NetworkPolicy{
		Name:          "example",
		ComponentName: v1alpha1.Webhook,
		Egress:        []networkingv1.NetworkPolicyEgressRule{},
	})
}

func TestCreateOrUpdateNetworkPolicy_CreatesWhenAbsent(t *testing.T) {
	c, kubeClient, recorder := newTestUserDefinedController()
	policy := desiredPolicy()

	require.NoError(t, c.createOrUpdateNetworkPolicy(context.Background(), policy))

	assert.Equal(t, 1, countActions(kubeClient.Actions(), "create", "networkpolicies"), "expected exactly one create")
	assert.Equal(t, 0, countActions(kubeClient.Actions(), "update", "networkpolicies"), "expected no update on create path")
	assert.Len(t, recorder.Events(), 1)
	assert.Equal(t, "NetworkPolicyCreated", recorder.Events()[0].Reason)
}

func TestCreateOrUpdateNetworkPolicy_NoOpWhenUnchanged(t *testing.T) {
	desired := desiredPolicy()
	// The already-existing object is identical to the desired one.
	c, kubeClient, recorder := newTestUserDefinedController(desired.DeepCopy())

	require.NoError(t, c.createOrUpdateNetworkPolicy(context.Background(), desired))

	assert.Equal(t, 0, countActions(kubeClient.Actions(), "update", "networkpolicies"),
		"identical policy must not trigger an update (avoids reconciliation loop)")
	assert.Equal(t, 0, countActions(kubeClient.Actions(), "create", "networkpolicies"))
	assert.Empty(t, recorder.Events(), "no event should be emitted on a no-op sync")
}

func TestCreateOrUpdateNetworkPolicy_UpdatesWhenSpecDrifts(t *testing.T) {
	existing := desiredPolicy()
	// Simulate drift: a different pod selector than the desired state.
	existing.Spec.PodSelector = metav1.LabelSelector{MatchLabels: map[string]string{"app": "stale"}}
	c, kubeClient, recorder := newTestUserDefinedController(existing.DeepCopy())

	require.NoError(t, c.createOrUpdateNetworkPolicy(context.Background(), desiredPolicy()))

	assert.Equal(t, 1, countActions(kubeClient.Actions(), "update", "networkpolicies"), "drifted spec must be reconciled")
	require.Len(t, recorder.Events(), 1)
	assert.Equal(t, "NetworkPolicyUpdated", recorder.Events()[0].Reason)
}

func TestCreateOrUpdateNetworkPolicy_UpdatesWhenLabelsDrift(t *testing.T) {
	existing := desiredPolicy()
	existing.Labels = map[string]string{"unexpected": "label"}
	c, kubeClient, _ := newTestUserDefinedController(existing.DeepCopy())

	require.NoError(t, c.createOrUpdateNetworkPolicy(context.Background(), desiredPolicy()))

	assert.Equal(t, 1, countActions(kubeClient.Actions(), "update", "networkpolicies"), "drifted labels must be reconciled")
}

func TestCreateOrUpdateNetworkPolicy_IsIdempotent(t *testing.T) {
	desired := desiredPolicy()
	c, kubeClient, _ := newTestUserDefinedController(desired.DeepCopy())

	// Multiple syncs against an already-correct policy must never update.
	for i := 0; i < 3; i++ {
		require.NoError(t, c.createOrUpdateNetworkPolicy(context.Background(), desired))
	}

	assert.Equal(t, 0, countActions(kubeClient.Actions(), "update", "networkpolicies"),
		"repeated syncs of an unchanged policy must not issue updates")
}
