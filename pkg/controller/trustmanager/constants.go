package trustmanager

import (
	"os"
	"time"
)

const (
	trustManagerCommonName = "cert-manager-trust-manager"

	ControllerName = trustManagerCommonName + "-controller"

	controllerProcessedAnnotation = "operator.openshift.io/trust-manager-processed"

	finalizer = "trustmanager.openshift.operator.io/" + ControllerName

	defaultRequeueTime = 30 * time.Second

	trustManagerObjectName = "cluster"

	trustManagerImageNameEnvVarName    = "RELATED_IMAGE_CERT_MANAGER_TRUST_MANAGER"
	trustManagerImageVersionEnvVarName = "TRUST_MANAGER_OPERAND_IMAGE_VERSION"

	defaultTrustNamespace = "cert-manager"
)

var controllerDefaultResourceLabels = map[string]string{
	"app":                          trustManagerCommonName,
	"app.kubernetes.io/name":       "trust-manager",
	"app.kubernetes.io/instance":   "trust-manager",
	"app.kubernetes.io/version":    os.Getenv(trustManagerImageVersionEnvVarName),
	"app.kubernetes.io/managed-by": "cert-manager-operator",
	"app.kubernetes.io/part-of":    "cert-manager-operator",
}

const (
	serviceAccountAssetName            = "trust-manager/trust-manager-serviceaccount.yaml"
	clusterRoleAssetName               = "trust-manager/trust-manager-clusterrole.yaml"
	clusterRoleBindingAssetName        = "trust-manager/trust-manager-clusterrolebinding.yaml"
	roleAssetName                      = "trust-manager/trust-manager-role.yaml"
	roleLeasesAssetName                = "trust-manager/trust-manager:leaderelection-role.yaml"
	roleBindingAssetName               = "trust-manager/trust-manager-rolebinding.yaml"
	roleBindingLeasesAssetName         = "trust-manager/trust-manager:leaderelection-rolebinding.yaml"
	serviceAssetName                   = "trust-manager/trust-manager-service.yaml"
	metricsServiceAssetName            = "trust-manager/trust-manager-metrics-service.yaml"
	deploymentAssetName                = "trust-manager/trust-manager-deployment.yaml"
	certificateAssetName               = "trust-manager/trust-manager-certificate.yaml"
	issuerAssetName                    = "trust-manager/trust-manager-issuer.yaml"
	validatingWebhookConfigurationName = "trust-manager/trust-manager-validatingwebhookconfiguration.yaml"
)
