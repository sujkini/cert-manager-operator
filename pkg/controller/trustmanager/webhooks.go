package trustmanager

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyValidatingWebhookConfiguration(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	desired := decodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(validatingWebhookConfigurationName))
	updateResourceLabels(desired, labels)
	updateResourceAnnotations(desired, annotations)
	updateWebhookCAInjection(desired, tm)
	updateWebhookClientConfigNamespace(desired, tm)
	return r.createOrUpdateObject(tm, desired, createRecon, "validatingwebhookconfiguration")
}

func updateWebhookCAInjection(webhook *admissionregistrationv1.ValidatingWebhookConfiguration, tm *v1alpha1.TrustManager) {
	annotations := webhook.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	ns := operandNamespace(tm)
	annotations["cert-manager.io/inject-ca-from"] = fmt.Sprintf("%s/trust-manager", ns)
	webhook.SetAnnotations(annotations)
}

func updateWebhookClientConfigNamespace(webhook *admissionregistrationv1.ValidatingWebhookConfiguration, tm *v1alpha1.TrustManager) {
	ns := operandNamespace(tm)
	for i := range webhook.Webhooks {
		if webhook.Webhooks[i].ClientConfig.Service != nil {
			webhook.Webhooks[i].ClientConfig.Service.Namespace = ns
		}
	}
}
