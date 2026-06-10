package trustmanager

import (
	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyCertificates(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	desired := decodeCertificateObjBytes(assets.MustAsset(certificateAssetName))
	updateNamespace(desired, operandNamespace(tm))
	updateResourceLabels(desired, labels)
	updateResourceAnnotations(desired, annotations)
	return r.createOrUpdateObject(tm, desired, createRecon, "certificate")
}
