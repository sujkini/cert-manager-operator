package trustmanager

import (
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyRBACResource(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	if err := r.createOrApplyClusterRole(tm, labels, annotations, createRecon); err != nil {
		return err
	}
	if err := r.createOrApplyClusterRoleBinding(tm, labels, annotations, createRecon); err != nil {
		return err
	}
	for _, asset := range []string{roleAssetName, roleLeasesAssetName} {
		role := decodeRoleObjBytes(assets.MustAsset(asset))
		updateNamespace(role, operandNamespace(tm))
		updateResourceLabels(role, labels)
		updateResourceAnnotations(role, annotations)
		if err := r.createOrUpdateObject(tm, role, createRecon, "role"); err != nil {
			return err
		}
	}
	for _, asset := range []string{roleBindingAssetName, roleBindingLeasesAssetName} {
		rb := decodeRoleBindingObjBytes(assets.MustAsset(asset))
		updateNamespace(rb, operandNamespace(tm))
		updateResourceLabels(rb, labels)
		updateResourceAnnotations(rb, annotations)
		if err := r.createOrUpdateObject(tm, rb, createRecon, "rolebinding"); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) createOrApplyClusterRole(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	desired := decodeClusterRoleObjBytes(assets.MustAsset(clusterRoleAssetName))
	updateResourceLabels(desired, labels)
	updateResourceAnnotations(desired, annotations)
	patchSecretTargetRules(desired, tm)
	return r.createOrUpdateObject(tm, desired, createRecon, "clusterrole")
}

func (r *Reconciler) createOrApplyClusterRoleBinding(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	desired := decodeClusterRoleBindingObjBytes(assets.MustAsset(clusterRoleBindingAssetName))
	updateResourceLabels(desired, labels)
	updateResourceAnnotations(desired, annotations)
	for i := range desired.Subjects {
		if desired.Subjects[i].Kind == "ServiceAccount" {
			desired.Subjects[i].Namespace = operandNamespace(tm)
		}
	}
	return r.createOrUpdateObject(tm, desired, createRecon, "clusterrolebinding")
}

func patchSecretTargetRules(clusterRole *rbacv1.ClusterRole, tm *v1alpha1.TrustManager) {
	if tm.Spec.SecretTargets == nil || !tm.Spec.SecretTargets.Enabled {
		return
	}
	if tm.Spec.SecretTargets.AuthorizedSecretsAll {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		})
		return
	}
	for _, secretName := range tm.Spec.SecretTargets.AuthorizedSecrets {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: []string{secretName},
			Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		})
	}
}
