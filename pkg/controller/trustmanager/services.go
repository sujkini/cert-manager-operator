package trustmanager

import (
	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyServices(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	for _, asset := range []string{serviceAssetName, metricsServiceAssetName} {
		svc := decodeServiceObjBytes(assets.MustAsset(asset))
		updateNamespace(svc, operandNamespace(tm))
		updateResourceLabels(svc, labels)
		updateResourceAnnotations(svc, annotations)
		if err := r.createOrUpdateObject(tm, svc, createRecon, "service"); err != nil {
			return err
		}
	}
	return nil
}
