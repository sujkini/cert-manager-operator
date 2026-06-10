package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&TrustManager{}, &TrustManagerList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// TrustManagerList is a list of TrustManager objects.
type TrustManagerList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []TrustManager `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=trustmanagers,scope=Cluster,categories={cert-manager-operator,trust-manager,trustmanager}
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels={"app.kubernetes.io/name=trust-manager", "app.kubernetes.io/part-of=cert-manager-operator"}

// TrustManager describes the configuration and information about the managed trust-manager operand.
// The name must be `cluster` to make TrustManager a cluster singleton.
//
// When TrustManager is enabled on a Tech Preview cluster, trust-manager is deployed as an operand
// managed by cert-manager-operator.
//
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="trustmanager is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="TrustManager"
type TrustManager struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the TrustManager.
	// +kubebuilder:validation:Required
	// +required
	Spec TrustManagerSpec `json:"spec"`

	// status is the most recently observed status of the TrustManager.
	// +kubebuilder:validation:Optional
	// +optional
	Status TrustManagerStatus `json:"status,omitempty"`
}

// TrustManagerSpec is the specification of the desired behavior of the TrustManager.
type TrustManagerSpec struct {
	// managementState indicates whether the trust-manager operand is enabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +kubebuilder:default=Disabled
	// +kubebuilder:validation:Optional
	// +optional
	ManagementState Mode `json:"managementState,omitempty"`

	// logLevel supports a value range as per Kubernetes logging guidelines.
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=5
	// +kubebuilder:validation:Optional
	// +optional
	LogLevel int32 `json:"logLevel,omitempty"`

	// logFormat specifies the output format for trust-manager logging.
	// +kubebuilder:validation:Enum=text;json
	// +kubebuilder:default=text
	// +kubebuilder:validation:Optional
	// +optional
	LogFormat string `json:"logFormat,omitempty"`

	// trustNamespace is the namespace used as the trust source for trust-manager.
	// The namespace must exist before trust-manager can reach Ready status.
	// +kubebuilder:default=cert-manager
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Optional
	// +optional
	TrustNamespace string `json:"trustNamespace,omitempty"`

	// secretTargets configures whether trust-manager may use Secrets as bundle sources or targets.
	// +kubebuilder:validation:Optional
	// +optional
	SecretTargets *TrustManagerSecretTargets `json:"secretTargets,omitempty"`

	// filterExpiredCertificates configures whether expired certificates are removed from synthesized bundles.
	// +kubebuilder:validation:Optional
	// +optional
	FilterExpiredCertificates *TrustManagerFilterExpiredCertificates `json:"filterExpiredCertificates,omitempty"`

	// controllerConfig configures labels and annotations applied to resources created by the controller.
	// +kubebuilder:validation:Optional
	// +optional
	ControllerConfig *TrustManagerControllerConfig `json:"controllerConfig,omitempty"`
}

// TrustManagerSecretTargets configures secret read/write permissions for trust-manager.
type TrustManagerSecretTargets struct {
	// enabled indicates whether secret targets are permitted.
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// authorizedSecretsAll grants read/write permission to all secrets across the cluster when enabled is true.
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	// +optional
	AuthorizedSecretsAll bool `json:"authorizedSecretsAll,omitempty"`

	// authorizedSecrets is the allow-list of secret names trust-manager may read and write across namespaces.
	// +listType=atomic
	// +kubebuilder:validation:MaxItems:=100
	// +kubebuilder:validation:Optional
	// +optional
	AuthorizedSecrets []string `json:"authorizedSecrets,omitempty"`
}

// TrustManagerFilterExpiredCertificates configures bundle filtering behavior.
type TrustManagerFilterExpiredCertificates struct {
	// enabled indicates whether expired certificates are filtered from bundles.
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// TrustManagerControllerConfig configures metadata applied to managed operand resources.
type TrustManagerControllerConfig struct {
	// labels to apply to resources created for the trust-manager deployment.
	// +mapType=granular
	// +kubebuilder:validation:MaxProperties:=20
	// +kubebuilder:validation:Optional
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations to apply to resources created for the trust-manager deployment.
	// +mapType=granular
	// +kubebuilder:validation:MaxProperties:=20
	// +kubebuilder:validation:Optional
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TrustManagerStatus is the most recently observed status of the TrustManager.
type TrustManagerStatus struct {
	ConditionalStatus `json:",inline,omitempty"`

	// trustManagerImage is the operand image reference observed during reconciliation.
	// +optional
	TrustManagerImage string `json:"trustManagerImage,omitempty"`

	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
