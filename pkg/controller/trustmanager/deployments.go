package trustmanager

import (
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
)

func (r *Reconciler) createOrApplyDeployments(tm *v1alpha1.TrustManager, labels, annotations map[string]string, createRecon bool) error {
	desired, err := r.getDeploymentObject(tm, labels, annotations)
	if err != nil {
		return err
	}
	if err := r.createOrUpdateObject(tm, desired, createRecon, "deployment"); err != nil {
		return err
	}
	return r.updateObservedStatus(tm, imageFromDeployment(desired))
}

func (r *Reconciler) getDeploymentObject(tm *v1alpha1.TrustManager, labels, annotations map[string]string) (*appsv1.Deployment, error) {
	deployment := decodeDeploymentObjBytes(assets.MustAsset(deploymentAssetName))
	updateNamespace(deployment, operandNamespace(tm))
	updateResourceLabels(deployment, labels)
	updateResourceAnnotations(deployment, annotations)
	updatePodTemplateLabels(deployment, labels)
	updateDeploymentArgs(deployment, tm)
	if err := r.updateOperandImage(deployment); err != nil {
		return nil, NewIrrecoverableError(err, "failed to resolve trust-manager operand image")
	}
	return deployment, nil
}

func updateDeploymentArgs(deployment *appsv1.Deployment, tm *v1alpha1.TrustManager) {
	args := []string{
		fmt.Sprintf("--log-format=%s", logFormat(tm)),
		fmt.Sprintf("--log-level=%d", logLevel(tm)),
		fmt.Sprintf("--trust-namespace=%s", operandNamespace(tm)),
	}
	if tm.Spec.FilterExpiredCertificates != nil && tm.Spec.FilterExpiredCertificates.Enabled {
		args = append(args, "--filter-expired-certificates=true")
	}
	replaceContainerArgs(deployment, args)
}

func replaceContainerArgs(deployment *appsv1.Deployment, desiredArgs []string) {
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name != "trust-manager" {
			continue
		}
		existing := deployment.Spec.Template.Spec.Containers[i].Args
		preserved := make([]string, 0, len(existing))
		for _, arg := range existing {
			if strings.HasPrefix(arg, "--log-format=") ||
				strings.HasPrefix(arg, "--log-level=") ||
				strings.HasPrefix(arg, "--trust-namespace=") ||
				strings.HasPrefix(arg, "--filter-expired-certificates=") {
				continue
			}
			preserved = append(preserved, arg)
		}
		deployment.Spec.Template.Spec.Containers[i].Args = append(desiredArgs, preserved...)
	}
}

func logFormat(tm *v1alpha1.TrustManager) string {
	if tm.Spec.LogFormat != "" {
		return tm.Spec.LogFormat
	}
	return "text"
}

func logLevel(tm *v1alpha1.TrustManager) int32 {
	if tm.Spec.LogLevel != 0 {
		return tm.Spec.LogLevel
	}
	return 1
}

func (r *Reconciler) updateOperandImage(deployment *appsv1.Deployment) error {
	image := os.Getenv(trustManagerImageNameEnvVarName)
	if image == "" {
		return fmt.Errorf("%s environment variable is not set", trustManagerImageNameEnvVarName)
	}
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == "trust-manager" {
			deployment.Spec.Template.Spec.Containers[i].Image = image
		}
	}
	return nil
}

func imageFromDeployment(deployment *appsv1.Deployment) string {
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "trust-manager" {
			return c.Image
		}
	}
	return ""
}
