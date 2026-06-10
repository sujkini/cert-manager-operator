package trustmanager

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	for _, add := range []func(*runtime.Scheme) error{
		appsv1.AddToScheme,
		corev1.AddToScheme,
		rbacv1.AddToScheme,
		certmanagerv1.AddToScheme,
		admissionregistrationv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			panic(err)
		}
	}
}

func operandNamespace(tm *v1alpha1.TrustManager) string {
	if tm.Spec.TrustNamespace != "" {
		return tm.Spec.TrustNamespace
	}
	return defaultTrustNamespace
}

func isManagementEnabled(tm *v1alpha1.TrustManager) bool {
	return tm.Spec.ManagementState == v1alpha1.Enabled
}

func (r *Reconciler) updateStatus(ctx context.Context, changed *v1alpha1.TrustManager) error {
	key := client.ObjectKeyFromObject(changed)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.TrustManager{}
		if err := r.Get(ctx, key, current); err != nil {
			return fmt.Errorf("failed to fetch trustmanager %q for status update: %w", key, err)
		}
		changed.Status.DeepCopyInto(&current.Status)
		if err := r.StatusUpdate(ctx, current); err != nil {
			return fmt.Errorf("failed to update trustmanager %q status: %w", key, err)
		}
		return nil
	})
}

func (r *Reconciler) addFinalizer(ctx context.Context, tm *v1alpha1.TrustManager) error {
	if controllerutil.ContainsFinalizer(tm, finalizer) {
		return nil
	}
	if !controllerutil.AddFinalizer(tm, finalizer) {
		return fmt.Errorf("failed to add finalizer on trustmanager %q", tm.GetName())
	}
	if err := r.UpdateWithRetry(ctx, tm); err != nil {
		return fmt.Errorf("failed to persist finalizer on trustmanager %q: %w", tm.GetName(), err)
	}
	updated := &v1alpha1.TrustManager{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(tm), updated); err != nil {
		return err
	}
	updated.DeepCopyInto(tm)
	return nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, tm *v1alpha1.TrustManager) error {
	if !controllerutil.RemoveFinalizer(tm, finalizer) {
		return nil
	}
	return r.UpdateWithRetry(ctx, tm)
}

func containsProcessedAnnotation(tm *v1alpha1.TrustManager) bool {
	_, ok := tm.GetAnnotations()[controllerProcessedAnnotation]
	return ok
}

func addProcessedAnnotation(tm *v1alpha1.TrustManager) bool {
	if containsProcessedAnnotation(tm) {
		return false
	}
	annotations := tm.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[controllerProcessedAnnotation] = "true"
	tm.SetAnnotations(annotations)
	return true
}

func validateTrustManagerConfig(tm *v1alpha1.TrustManager) error {
	if !isManagementEnabled(tm) {
		return nil
	}
	ns := operandNamespace(tm)
	if ns == "" {
		return fmt.Errorf("spec.trustNamespace cannot be empty when managementState is Enabled")
	}
	if tm.Spec.SecretTargets != nil && tm.Spec.SecretTargets.Enabled {
		if !tm.Spec.SecretTargets.AuthorizedSecretsAll && len(tm.Spec.SecretTargets.AuthorizedSecrets) == 0 {
			return fmt.Errorf("spec.secretTargets.enabled is true but neither authorizedSecretsAll nor authorizedSecrets are configured")
		}
	}
	return nil
}

func (r *Reconciler) validateTrustNamespaceExists(tm *v1alpha1.TrustManager) error {
	ns := &corev1.Namespace{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: operandNamespace(tm)}, ns); err != nil {
		return NewRetryRequiredError(err, "trust namespace %q does not exist yet", operandNamespace(tm))
	}
	return nil
}

func mergeResourceLabels(tm *v1alpha1.TrustManager) map[string]string {
	labels := map[string]string{}
	if tm.Spec.ControllerConfig != nil {
		maps.Copy(labels, tm.Spec.ControllerConfig.Labels)
	}
	maps.Copy(labels, controllerDefaultResourceLabels)
	return labels
}

func mergeResourceAnnotations(tm *v1alpha1.TrustManager) map[string]string {
	if tm.Spec.ControllerConfig == nil {
		return nil
	}
	return maps.Clone(tm.Spec.ControllerConfig.Annotations)
}

func updateNamespace(obj client.Object, namespace string) {
	obj.SetNamespace(namespace)
}

func updateResourceLabels(obj client.Object, labels map[string]string) {
	current := obj.GetLabels()
	if current == nil {
		current = map[string]string{}
	}
	maps.Copy(current, labels)
	obj.SetLabels(current)
}

func updateResourceAnnotations(obj client.Object, annotations map[string]string) {
	if len(annotations) == 0 {
		return
	}
	current := obj.GetAnnotations()
	if current == nil {
		current = map[string]string{}
	}
	maps.Copy(current, annotations)
	obj.SetAnnotations(current)
}

func updatePodTemplateLabels(deployment *appsv1.Deployment, labels map[string]string) {
	podLabels := deployment.Spec.Template.Labels
	if podLabels == nil {
		podLabels = map[string]string{}
	}
	maps.Copy(podLabels, labels)
	deployment.Spec.Template.Labels = podLabels
}

func hasObjectChanged(desired, current client.Object) bool {
	return !reflect.DeepEqual(desired.GetLabels(), current.GetLabels()) ||
		!reflect.DeepEqual(desired.GetAnnotations(), current.GetAnnotations()) ||
		!reflect.DeepEqual(desired, current)
}

func decodeObjBytes[T client.Object](bytes []byte) T {
	obj := reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(T)
	if _, _, err := codecs.UniversalDecoder().Decode(bytes, nil, obj); err != nil {
		panic(fmt.Sprintf("failed to decode %T: %v", obj, err))
	}
	return obj
}

func decodeDeploymentObjBytes(b []byte) *appsv1.Deployment {
	return decodeObjBytes[*appsv1.Deployment](b)
}
func decodeServiceAccountObjBytes(b []byte) *corev1.ServiceAccount {
	return decodeObjBytes[*corev1.ServiceAccount](b)
}
func decodeServiceObjBytes(b []byte) *corev1.Service { return decodeObjBytes[*corev1.Service](b) }
func decodeClusterRoleObjBytes(b []byte) *rbacv1.ClusterRole {
	return decodeObjBytes[*rbacv1.ClusterRole](b)
}
func decodeClusterRoleBindingObjBytes(b []byte) *rbacv1.ClusterRoleBinding {
	return decodeObjBytes[*rbacv1.ClusterRoleBinding](b)
}
func decodeRoleObjBytes(b []byte) *rbacv1.Role { return decodeObjBytes[*rbacv1.Role](b) }
func decodeRoleBindingObjBytes(b []byte) *rbacv1.RoleBinding {
	return decodeObjBytes[*rbacv1.RoleBinding](b)
}
func decodeCertificateObjBytes(b []byte) *certmanagerv1.Certificate {
	return decodeObjBytes[*certmanagerv1.Certificate](b)
}
func decodeIssuerObjBytes(b []byte) *certmanagerv1.Issuer {
	return decodeObjBytes[*certmanagerv1.Issuer](b)
}
func decodeValidatingWebhookConfigurationObjBytes(b []byte) *admissionregistrationv1.ValidatingWebhookConfiguration {
	return decodeObjBytes[*admissionregistrationv1.ValidatingWebhookConfiguration](b)
}

func (r *Reconciler) createOrUpdateObject(tm *v1alpha1.TrustManager, desired client.Object, createRecon bool, resourceType string) error {
	name := fmt.Sprintf("%s/%s", desired.GetNamespace(), desired.GetName())
	fetched := reflect.New(reflect.TypeOf(desired).Elem()).Interface().(client.Object)
	exist, err := r.Exists(r.ctx, client.ObjectKeyFromObject(desired), fetched)
	if err != nil {
		return FromClientError(err, "failed to check %s %s already exists", name, resourceType)
	}
	if exist && createRecon {
		r.eventRecorder.Eventf(tm, corev1.EventTypeWarning, "ResourceAlreadyExists", "%s %s already exists, maybe from previous installation", resourceType, name)
	}
	if exist && hasObjectChanged(desired, fetched) {
		if err := r.UpdateWithRetry(r.ctx, desired); err != nil {
			return FromClientError(err, "failed to update %s %s", resourceType, name)
		}
		r.eventRecorder.Eventf(tm, corev1.EventTypeNormal, "Reconciled", "%s %s reconciled", resourceType, name)
	}
	if !exist {
		if err := r.Create(r.ctx, desired); err != nil {
			return FromClientError(err, "failed to create %s %s", resourceType, name)
		}
		r.eventRecorder.Eventf(tm, corev1.EventTypeNormal, "Reconciled", "%s %s created", resourceType, name)
	}
	return nil
}

func (r *Reconciler) updateObservedStatus(tm *v1alpha1.TrustManager, image string) error {
	changed := false
	if tm.Status.TrustManagerImage != image {
		tm.Status.TrustManagerImage = image
		changed = true
	}
	if tm.Status.ObservedGeneration != tm.GetGeneration() {
		tm.Status.ObservedGeneration = tm.GetGeneration()
		changed = true
	}
	if !changed {
		return nil
	}
	return r.updateStatus(r.ctx, tm)
}

func setReadyConditions(tm *v1alpha1.TrustManager, ready metav1.ConditionStatus, reason, message string) {
	tm.Status.SetCondition(v1alpha1.Degraded, metav1.ConditionFalse, v1alpha1.ReasonReady, "")
	tm.Status.SetCondition(v1alpha1.Ready, ready, reason, message)
}
