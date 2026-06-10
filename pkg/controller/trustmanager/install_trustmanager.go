package trustmanager

import (
	"fmt"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

func (r *Reconciler) reconcileTrustManagerOperand(tm *v1alpha1.TrustManager, createRecon bool) error {
	if err := validateTrustManagerConfig(tm); err != nil {
		return NewIrrecoverableError(err, "trustmanager %q configuration validation failed", tm.GetName())
	}
	if err := r.validateTrustNamespaceExists(tm); err != nil {
		return err
	}

	resourceLabels := mergeResourceLabels(tm)
	resourceAnnotations := mergeResourceAnnotations(tm)

	if err := r.createOrApplyServiceAccounts(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyRBACResource(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyIssuers(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyCertificates(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyServices(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyDeployments(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyValidatingWebhookConfiguration(tm, resourceLabels, resourceAnnotations, createRecon); err != nil {
		return err
	}

	if addProcessedAnnotation(tm) {
		if err := r.UpdateWithRetry(r.ctx, tm); err != nil {
			return fmt.Errorf("failed to update processed annotation on trustmanager %q: %w", tm.GetName(), err)
		}
	}
	return nil
}
