package trustmanager

import (
	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyIssuers(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	desired := decodeIssuerObjBytes(assets.MustAsset(issuerAssetName))
	updateNamespace(desired, operandNamespace(tm))
	updateResourceLabels(desired, labels)
	updateResourceAnnotations(desired, annotations)
	return r.createOrUpdateObject(tm, desired, createRecon, "issuer")
}
