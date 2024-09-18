package webhook

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/evanphx/json-patch/v5"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type mockOpenAIClient struct {
	response string
	err      error
}

func (m *mockOpenAIClient) CreateChatCompletion(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}

// Mock the getCRD function to return a predefined CRD without calling the Kubernetes API.
func mockGetCRD(cr *unstructured.Unstructured) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdYAML := `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusterextensions.olm.operatorframework.io
spec:
  group: olm.operatorframework.io
  names:
    kind: ClusterExtension
    plural: clusterextensions
    singular: clusterextension
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                install:
                  type: object
                  properties:
                    namespace:
                      type: string
                    serviceAccount:
                      type: object
                      properties:
                        name:
                          type: string
                source:
                  type: object
                  properties:
                    sourceType:
                      type: string
                    catalog:
                      type: object
                      properties:
                        packageName:
                          type: string
`
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := yaml.Unmarshal([]byte(crdYAML), crd)
	if err != nil {
		return nil, err
	}
	return crd, nil
}

func TestAdjustCRWithLLM_Success(t *testing.T) {
	// Prepare the CR YAML
	crYAML := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      packageName: example-package
`
	// Convert CR YAML to JSON
	crJSON, err := yaml.YAMLToJSON([]byte(crYAML))
	if err != nil {
		t.Fatalf("Failed to convert CR YAML to JSON: %v", err)
	}

	// Unmarshal JSON into unstructured.Unstructured
	cr := &unstructured.Unstructured{}
	err = cr.UnmarshalJSON(crJSON)
	if err != nil {
		t.Fatalf("Failed to unmarshal CR JSON: %v", err)
	}

	// Prepare the CRD
	crd, err := mockGetCRD(cr)
	if err != nil {
		t.Fatalf("Failed to get CRD: %v", err)
	}

	// Prepare the mock OpenAI client
	mockResponse := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      packageName: corrected-package
`
	mockClient := &mockOpenAIClient{
		response: mockResponse,
		err:      nil,
	}

	// Call AdjustCRWithLLM
	adjustedCR, err := AdjustCRWithLLM(cr, crd, mockClient)
	if err != nil {
		t.Fatalf("AdjustCRWithLLM failed: %v", err)
	}

	// Validate the adjusted CR
	isValid, validationErrors := ValidateCR(adjustedCR)
	if !isValid {
		t.Fatalf("Adjusted CR is invalid: %s", validationErrors)
	}

	// Verify that the adjusted CR contains the expected changes
	packageName, found, err := unstructured.NestedString(adjustedCR.Object, "spec", "source", "catalog", "packageName")
	if err != nil || !found {
		t.Fatalf("packageName not found in adjusted CR")
	}
	if packageName != "corrected-package" {
		t.Errorf("expected packageName to be 'corrected-package', got '%s'", packageName)
	}
}

func TestAdjustCRWithLLM_OpenAIError(t *testing.T) {
	// Prepare the CR
	crYAML := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      packageName: example-package
`
	cr := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(crYAML), &cr.Object)
	if err != nil {
		t.Fatalf("Failed to unmarshal CR: %v", err)
	}

	// Prepare the CRD
	crd, err := mockGetCRD(cr)
	if err != nil {
		t.Fatalf("Failed to get CRD: %v", err)
	}

	// Prepare the mock OpenAI client that returns an error
	mockClient := &mockOpenAIClient{
		response: "",
		err:      fmt.Errorf("OpenAI API error"),
	}

	// Call AdjustCRWithLLM
	adjustedCR, err := AdjustCRWithLLM(cr, crd, mockClient)
	if err == nil {
		t.Fatalf("Expected AdjustCRWithLLM to fail, but it succeeded")
	}

	if adjustedCR != nil {
		t.Errorf("Expected adjustedCR to be nil when error occurs")
	}
}

func TestAdjustCRWithLLM_InvalidAdjustedCR(t *testing.T) {
	// Prepare the CR
	crYAML := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      packageName: example-package
`
	cr := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(crYAML), &cr.Object)
	if err != nil {
		t.Fatalf("Failed to unmarshal CR: %v", err)
	}

	// Prepare the CRD
	crd, err := mockGetCRD(cr)
	if err != nil {
		t.Fatalf("Failed to get CRD: %v", err)
	}

	// Prepare the mock OpenAI client that returns an invalid CR
	mockResponse := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      invalidField: "some value"
`
	mockClient := &mockOpenAIClient{
		response: mockResponse,
		err:      nil,
	}

	// Call AdjustCRWithLLM
	adjustedCR, err := AdjustCRWithLLM(cr, crd, mockClient)
	if err != nil {
		t.Fatalf("AdjustCRWithLLM failed: %v", err)
	}

	// Validate the adjusted CR
	isValid, validationErrors := ValidateCR(adjustedCR)
	if isValid {
		t.Fatalf("Expected adjusted CR to be invalid, but it is valid")
	}

	t.Logf("Adjusted CR is invalid as expected: %s", validationErrors)
}

func TestMutate_Success(t *testing.T) {
	// Prepare the CR YAML
	crYAML := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      # packageName is missing
`
	// Convert CR YAML to JSON
	crJSON, err := yaml.YAMLToJSON([]byte(crYAML))
	if err != nil {
		t.Fatalf("Failed to convert CR YAML to JSON: %v", err)
	}

	// Create the AdmissionReview request
	admissionReview := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Object: runtime.RawExtension{
				Raw: crJSON,
			},
			Operation: admissionv1.Create,
		},
	}

	// Prepare the mock OpenAI client
	mockResponse := `
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: example
spec:
  install:
    namespace: example-namespace
    serviceAccount:
      name: example-sa
  source:
    sourceType: Catalog
    catalog:
      packageName: corrected-package
`
	mockClient := &mockOpenAIClient{
		response: mockResponse,
		err:      nil,
	}

	// Replace the original getCRD function with the mock
	originalGetCRD := getCRD
	defer func() { getCRD = originalGetCRD }()
	getCRD = mockGetCRD

	// Call mutate
	admissionResponse := mutate(&admissionReview, mockClient)

	// Check the response
	if !admissionResponse.Allowed {
		t.Fatalf("Expected admission response to be allowed")
	}

	//debug
	fmt.Printf("AdmissionResponse.Patch: %s\n", string(admissionResponse.Patch))

	// Apply the patch to the original CR
	patchedCRJSON, err := applyJSONPatch(crJSON, admissionResponse.Patch)
	if err != nil {
		t.Fatalf("Failed to apply patch: %v", err)
	}

	patchedCR := &unstructured.Unstructured{}
	err = patchedCR.UnmarshalJSON(patchedCRJSON)
	if err != nil {
		t.Fatalf("Failed to unmarshal patched CR: %v", err)
	}

	// Validate the patched CR
	isValid, validationErrors := ValidateCR(patchedCR)
	if !isValid {
		t.Fatalf("Patched CR is invalid: %s", validationErrors)
	}

	// Verify that the patched CR contains the expected changes
	packageName, found, err := unstructured.NestedString(patchedCR.Object, "spec", "source", "catalog", "packageName")
	if err != nil || !found {
		t.Fatalf("packageName not found in patched CR")
	}
	if packageName != "corrected-package" {
		t.Errorf("expected packageName to be 'corrected-package', got '%s'", packageName)
	}
}

// Apply a JSON patch to the original JSON to get the modified JSON
func applyJSONPatch(originalJSON, patchBytes []byte) ([]byte, error) {
	fmt.Printf("Applying JSON Patch:\n%s\n", string(patchBytes)) // debug

	// Decode the JSON Patch
	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON Patch: %v", err)
	}

	// Apply the JSON Patch to the original JSON
	modifiedJSON, err := patch.Apply(originalJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to apply JSON Patch: %v", err)
	}

	return modifiedJSON, nil
}

// Helper function to replace a value in the nested map based on JSON Pointer path
func replaceValue(obj *map[string]interface{}, path string, value interface{}) {
	// Split the path and navigate to the field
	fields := strings.Split(strings.TrimPrefix(path, "/"), "/")
	current := *obj
	for i, field := range fields {
		if i == len(fields)-1 {
			current[field] = value
		} else {
			if next, ok := current[field].(map[string]interface{}); ok {
				current = next
			} else {
				// Create nested map if it doesn't exist
				newMap := make(map[string]interface{})
				current[field] = newMap
				current = newMap
			}
		}
	}
}
