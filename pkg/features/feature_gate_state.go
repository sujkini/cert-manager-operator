package features

import (
	"context"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

const (
	clusterFeatureGateName = "cluster"
	clusterPreviewRetries  = 3
	clusterPreviewBackoff  = 30 * time.Second
)

var previewFeatureSets = map[configv1.FeatureSet]struct{}{
	configv1.TechPreviewNoUpgrade: {},
	configv1.CustomNoUpgrade:      {},
	configv1.DevPreviewNoUpgrade:  {},
}

type clusterFeatureGateGetter interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*configv1.FeatureGate, error)
}

// FeatureGateState evaluates TechPreview feature gates against cluster FeatureSet.
type FeatureGateState struct {
	featureGates clusterFeatureGateGetter
}

// NewFeatureGateState returns a FeatureGateState backed by the OpenShift config client.
func NewFeatureGateState(configClient configv1client.Interface) *FeatureGateState {
	return newFeatureGateState(configClient.ConfigV1().FeatureGates())
}

func newFeatureGateState(featureGates clusterFeatureGateGetter) *FeatureGateState {
	return &FeatureGateState{featureGates: featureGates}
}

// IsTrustManagerFeatureGateEnabled reports whether TrustManager is enabled at runtime.
func (s *FeatureGateState) IsTrustManagerFeatureGateEnabled(ctx context.Context) bool {
	if !DefaultFeatureGate.Enabled(v1alpha1.FeatureTrustManager) {
		return false
	}
	return s.passesClusterPreviewGating(ctx)
}

func (s *FeatureGateState) passesClusterPreviewGating(ctx context.Context) bool {
	for attempt := 0; attempt < clusterPreviewRetries; attempt++ {
		fg, err := s.featureGates.Get(ctx, clusterFeatureGateName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false
			}
			if attempt < clusterPreviewRetries-1 {
				time.Sleep(clusterPreviewBackoff)
				continue
			}
			return false
		}
		_, ok := previewFeatureSets[fg.Spec.FeatureSet]
		return ok
	}
	return false
}
