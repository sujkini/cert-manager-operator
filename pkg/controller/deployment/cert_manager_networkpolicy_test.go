package deployment

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"

	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

// newTestController builds a controller wired to a fake kube client (optionally
// pre-populated with the given policies) and an in-memory event recorder.
func newTestController(objs ...runtime.Object) (*CertManagerNetworkPolicyUserDefinedController, *fake.Clientset) {
	kubeClient := fake.NewSimpleClientset(objs...)
	c := &CertManagerNetworkPolicyUserDefinedController{
		kubeClient:    kubeClient,
		eventRecorder: events.NewInMemoryRecorder("test", clock.RealClock{}),
	}
	return c, kubeClient
}

// countActions returns the number of client actions matching the given verb on
// networkpolicies.
func countActions(client *fake.Clientset, verb string) int {
	n := 0
	for _, a := range client.Actions() {
		if a.GetVerb() == verb && a.GetResource().Resource == "networkpolicies" {
			n++
		}
	}
	return n
}

// samplePolicy returns a deterministic user-defined NetworkPolicy for testing,
// built through the controller's own conversion so the test exercises the real
// desired-state shape.
func samplePolicy() *networkingv1.NetworkPolicy {
	port := intstr.FromInt(443)
	proto := corev1.ProtocolTCP
	c := &CertManagerNetworkPolicyUserDefinedController{}
	return c.createUserNetworkPolicy(v1alpha1.NetworkPolicy{
		Name:          "sample",
		ComponentName: v1alpha1.CoreController,
		Egress: []networkingv1.NetworkPolicyEgressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{
					{Port: &port, Protocol: &proto},
				},
			},
		},
	})
}

// TestCreateOrUpdateNetworkPolicy_Creates verifies a Create is issued when the
// policy does not yet exist.
func TestCreateOrUpdateNetworkPolicy_Creates(t *testing.T) {
	c, client := newTestController()

	if err := c.createOrUpdateNetworkPolicy(context.Background(), samplePolicy()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := countActions(client, "create"); got != 1 {
		t.Errorf("expected 1 create action, got %d", got)
	}
	if got := countActions(client, "update"); got != 0 {
		t.Errorf("expected 0 update actions, got %d", got)
	}
}

// TestCreateOrUpdateNetworkPolicy_NoUpdateWhenUnchanged is the regression test
// for CM-763: an already up-to-date policy must not be updated again, otherwise
// the controller enters an infinite reconcile/update loop.
func TestCreateOrUpdateNetworkPolicy_NoUpdateWhenUnchanged(t *testing.T) {
	existing := samplePolicy()
	c, client := newTestController(existing)

	// Reconcile with an identical desired policy.
	if err := c.createOrUpdateNetworkPolicy(context.Background(), samplePolicy()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := countActions(client, "update"); got != 0 {
		t.Errorf("expected 0 update actions for an unchanged policy, got %d", got)
	}
}

// TestCreateOrUpdateNetworkPolicy_UpdatesOnSpecChange verifies an Update is
// issued when the desired spec differs from the existing object.
func TestCreateOrUpdateNetworkPolicy_UpdatesOnSpecChange(t *testing.T) {
	existing := samplePolicy()
	c, client := newTestController(existing)

	desired := samplePolicy()
	newPort := intstr.FromInt(8200)
	proto := corev1.ProtocolTCP
	desired.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
		{Ports: []networkingv1.NetworkPolicyPort{{Port: &newPort, Protocol: &proto}}},
	}

	if err := c.createOrUpdateNetworkPolicy(context.Background(), desired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := countActions(client, "update"); got != 1 {
		t.Errorf("expected 1 update action for a changed policy, got %d", got)
	}
}

// TestCreateOrUpdateNetworkPolicy_UpdatesOnLabelChange verifies label drift is
// reconciled via an Update.
func TestCreateOrUpdateNetworkPolicy_UpdatesOnLabelChange(t *testing.T) {
	existing := samplePolicy()
	existing.Labels = map[string]string{} // drifted labels
	c, client := newTestController(existing)

	if err := c.createOrUpdateNetworkPolicy(context.Background(), samplePolicy()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := countActions(client, "update"); got != 1 {
		t.Errorf("expected 1 update action for drifted labels, got %d", got)
	}
}
