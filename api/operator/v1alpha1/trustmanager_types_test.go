package v1alpha1

import (
	"os"
	"path"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

const (
	trustManagerCRDFile     = "operator.openshift.io_trustmanagers.yaml"
	trustManagerCRDFilePath = "../../../config/crd/bases"
)

func TestTrustManagerStatusDefault(t *testing.T) {
	filepath := path.Join(trustManagerCRDFilePath, trustManagerCRDFile)
	trustManagerCRDBytes, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read trustmanager CRD file %q: %v", filepath, err)
	}

	var trustManagerCRD map[string]any
	if err := yaml.Unmarshal(trustManagerCRDBytes, &trustManagerCRD); err != nil {
		t.Fatalf("failed to unmarshal trustmanager CRD: %v", err)
	}
	trustManagerCRDSpec := trustManagerCRD["spec"].(map[string]any)
	trustManagerCRDVersions := trustManagerCRDSpec["versions"].([]any)
	for _, v := range trustManagerCRDVersions {
		trustManagerCRDVersion := v.(map[string]any)
		status, exists, err := unstructured.NestedMap(trustManagerCRDVersion, "schema", "openAPIV3Schema", "properties", "status")
		if err != nil {
			t.Fatalf("failed to get nested map: %v", err)
		}

		if !exists {
			t.Fatalf("status field does not exist under the CRD")
		}

		if _, ok := status["default"]; ok {
			t.Fatalf("expected no default for the trustmanager CRD status")
		}
	}
}

func TestTrustManagerManagementStateDefault(t *testing.T) {
	tm := TrustManager{}
	if tm.Spec.ManagementState != "" {
		t.Fatalf("expected empty default managementState on Go type, got %q", tm.Spec.ManagementState)
	}
}
