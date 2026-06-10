package features

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

type fakeFeatureGates struct {
	featureSet configv1.FeatureSet
}

func (f *fakeFeatureGates) Get(_ context.Context, name string, _ metav1.GetOptions) (*configv1.FeatureGate, error) {
	return &configv1.FeatureGate{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: configv1.FeatureGateSpec{
			FeatureGateSelection: configv1.FeatureGateSelection{
				FeatureSet: f.featureSet,
			},
		},
	}, nil
}

func TestPassesClusterPreviewGating(t *testing.T) {
	tests := []struct {
		name       string
		featureSet configv1.FeatureSet
		want       bool
	}{
		{name: "tech preview", featureSet: configv1.TechPreviewNoUpgrade, want: true},
		{name: "custom no upgrade", featureSet: configv1.CustomNoUpgrade, want: true},
		{name: "dev preview", featureSet: configv1.DevPreviewNoUpgrade, want: true},
		{name: "default", featureSet: configv1.Default, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newFeatureGateState(&fakeFeatureGates{featureSet: tt.featureSet})
			assert.Equal(t, tt.want, state.passesClusterPreviewGating(context.Background()))
		})
	}
}

func TestIsTrustManagerFeatureGateEnabled(t *testing.T) {
	state := newFeatureGateState(&fakeFeatureGates{featureSet: configv1.TechPreviewNoUpgrade})

	require.NoError(t, mutableFeatureGate.Set(string(v1alpha1.FeatureTrustManager)+"=true"))
	assert.True(t, state.IsTrustManagerFeatureGateEnabled(context.Background()))

	require.NoError(t, mutableFeatureGate.Set(string(v1alpha1.FeatureTrustManager)+"=false"))
	assert.False(t, state.IsTrustManagerFeatureGateEnabled(context.Background()))
}
