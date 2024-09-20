package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/wI2L/jsondiff"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"log"
	"net/http"
	"os"
	"sigs.k8s.io/yaml"
	"strings"
)

var (
	scheme       = runtime.NewScheme()
	codecs       = serializer.NewCodecFactory(scheme)
	deserializer = codecs.UniversalDeserializer()
)

// openaiClientInterface defines the methods used from the OpenAI client.
type openaiClientInterface interface {
	CreateChatCompletion(ctx context.Context, prompt string) (string, error)
}

// openAIClient is a wrapper around the OpenAI client.
type openAIClient struct {
	client *openai.Client
}

func (c *openAIClient) CreateChatCompletion(ctx context.Context, prompt string) (string, error) {
	chatCompletion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		}),
		Model: openai.F("gpt-4o"), // Use "gpt-3.5-turbo" if "gpt-4o" is not accessible
	})
	if err != nil {
		return "", err
	}

	if len(chatCompletion.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return chatCompletion.Choices[0].Message.Content, nil
}

// Mutate handles the admission review requests
func Mutate(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Invalid Content-Type, expected application/json", http.StatusUnsupportedMediaType)
		return
	}

	var admissionReview admissionv1.AdmissionReview
	if _, _, err := deserializer.Decode(body, nil, &admissionReview); err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, fmt.Sprintf("Could not decode body: %v", err), http.StatusBadRequest)
		return
	}

	// Initialize the OpenAI client
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Printf("OPENAI_API_KEY environment variable not set")
		http.Error(w, "OPENAI_API_KEY environment variable not set", http.StatusInternalServerError)
		return
	}
	client := &openAIClient{
		client: openai.NewClient(
			option.WithAPIKey(openaiAPIKey),
		),
	}

	// Process the AdmissionRequest
	admissionResponse := mutate(&admissionReview, client)

	// Send response
	admissionReview.Response = admissionResponse
	admissionReview.Response.UID = admissionReview.Request.UID

	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		log.Printf("Could not encode response: %v", err)
		http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(respBytes)
	if writeErr != nil {
		log.Printf("Error while sending response: %v", writeErr)
		return
	}
}

func mutate(ar *admissionv1.AdmissionReview, client openaiClientInterface) *admissionv1.AdmissionResponse {
	req := ar.Request

	// Only process create and update operations
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	// Decode the object
	raw := req.Object.Raw
	cr := &unstructured.Unstructured{}
	if _, _, err := deserializer.Decode(raw, nil, cr); err != nil {
		log.Printf("Could not decode raw object: %v", err)
		return toAdmissionResponse(err)
	}

	// Validate the CR
	isValid, validationErrors := ValidateCR(cr)
	if isValid {
		// CR is valid, allow it
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	log.Printf("CR is invalid: %s", validationErrors)

	crd, err := getCRD(cr)
	if err != nil {
		log.Printf("Failed to retrieve CRD: %v", err)
		return toAdmissionResponse(err)
	}

	// Adjust the CR using OpenAI
	adjustedCR, err := AdjustCRWithLLM(cr, crd, client)
	if err != nil {
		log.Printf("Failed to adjust CR with LLM: %v", err)
		return toAdmissionResponse(err)
	}

	// Validate the adjusted CR
	isValid, validationErrors = ValidateCR(adjustedCR)
	if !isValid {
		log.Printf("Adjusted CR is still invalid: %s", validationErrors)
		return toAdmissionResponse(fmt.Errorf("adjusted CR is still invalid: %s", validationErrors))
	}

	// Create a patch
	originalJSON, err := json.Marshal(cr.Object)
	if err != nil {
		return toAdmissionResponse(err)
	}
	adjustedJSON, err := json.Marshal(adjustedCR.Object)
	if err != nil {
		return toAdmissionResponse(err)
	}
	patchBytes, err := createJSONPatch(originalJSON, adjustedJSON)
	if err != nil {
		return toAdmissionResponse(err)
	}

	// Return the patch in the admission response
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func AdjustCRWithLLM(cr *unstructured.Unstructured, crd *apiextensionsv1.CustomResourceDefinition, client openaiClientInterface) (*unstructured.Unstructured, error) {
	// Convert CR to YAML
	crYAML, err := yaml.Marshal(cr.Object)
	if err != nil {
		log.Printf("Error marshalling CR to YAML: %v", err)
		return nil, err
	}
	log.Printf("CR YAML:\n%s\n", string(crYAML))

	// Convert CRD to YAML
	crdYAML, err := yaml.Marshal(crd)
	if err != nil {
		log.Printf("Error marshalling CRD to YAML: %v", err)
		return nil, err
	}
	log.Printf("CRD YAML:\n%s\n", string(crdYAML))

	// Construct the prompt to send to OpenAI
	prompt := fmt.Sprintf(`You are an expert in Kubernetes custom resources. Given the following Custom Resource Definition (CRD):

---
%s
---

And the following Custom Resource (CR) that may not conform to the CRD:

---
%s
---

Please adjust the CR so that it conforms to the CRD schema. Return only the corrected CR in YAML format. Do not include any explanations or additional text.`, string(crdYAML), string(crYAML))

	log.Printf("Generated OpenAI prompt:\n%s\n", prompt)

	// Call the OpenAI client
	adjustedCRYAML, err := client.CreateChatCompletion(context.TODO(), prompt)
	if err != nil {
		log.Printf("Error from OpenAI API: %v", err)
		return nil, err
	}
	log.Printf("Raw Adjusted CR YAML from OpenAI:\n%s\n", adjustedCRYAML)

	// Strip the ```yaml wrapper if present
	adjustedCRYAML = strings.TrimSpace(adjustedCRYAML)
	if strings.HasPrefix(adjustedCRYAML, "```yaml") && strings.HasSuffix(adjustedCRYAML, "```") {
		adjustedCRYAML = strings.TrimPrefix(adjustedCRYAML, "```yaml")
		adjustedCRYAML = strings.TrimSuffix(adjustedCRYAML, "```")
		adjustedCRYAML = strings.TrimSpace(adjustedCRYAML) // Remove any extra whitespace
	}
	log.Printf("Adjusted CR YAML after removing wrapper:\n%s\n", adjustedCRYAML)

	// Convert YAML to JSON
	adjustedCRJSON, err := yaml.YAMLToJSON([]byte(adjustedCRYAML))
	if err != nil {
		log.Printf("Failed to convert adjusted CR YAML to JSON: %v", err)
		log.Printf("Raw adjusted CR YAML:\n%s\n", adjustedCRYAML) // Debug the YAML that failed
		return nil, fmt.Errorf("failed to convert adjusted CR YAML to JSON: %v", err)
	}
	log.Printf("Adjusted CR JSON:\n%s\n", string(adjustedCRJSON))

	// Unmarshal JSON into unstructured.Unstructured
	adjustedCR := &unstructured.Unstructured{}
	err = adjustedCR.UnmarshalJSON(adjustedCRJSON)
	if err != nil {
		log.Printf("Failed to unmarshal adjusted CR JSON: %v", err)
		return nil, fmt.Errorf("failed to unmarshal adjusted CR JSON: %v", err)
	}

	return adjustedCR, nil
}

func ValidateCR(cr *unstructured.Unstructured) (bool, string) {
	// Example validation logic
	spec, found, err := unstructured.NestedMap(cr.Object, "spec")
	if err != nil || !found {
		return false, "spec field is missing"
	}

	// Add logging to check what's inside spec
	fmt.Printf("Validating CR Spec: %+v\n", spec)

	install, found, err := unstructured.NestedMap(spec, "install")
	if err != nil || !found {
		return false, "install field is missing"
	}

	// Validate install.namespace
	_, found, err = unstructured.NestedString(install, "namespace")
	if err != nil || !found {
		return false, "namespace is missing in install"
	}

	// Validate install.serviceAccount
	serviceAccount, found, err := unstructured.NestedMap(install, "serviceAccount")
	if err != nil || !found {
		return false, "serviceAccount is missing in install"
	}

	_, found, err = unstructured.NestedString(serviceAccount, "name")
	if err != nil || !found {
		return false, "serviceAccount name is missing"
	}

	// Validate source
	source, found, err := unstructured.NestedMap(spec, "source")
	if err != nil || !found {
		return false, "source field is missing"
	}

	// Validate source.sourceType
	_, found, err = unstructured.NestedString(source, "sourceType")
	if err != nil || !found {
		return false, "sourceType is missing in source"
	}

	// Validate source.catalog.packageName
	catalog, found, err := unstructured.NestedMap(source, "catalog")
	if err != nil || !found {
		return false, "catalog is missing in source"
	}

	_, found, err = unstructured.NestedString(catalog, "packageName")
	if err != nil || !found {
		return false, "packageName is missing in catalog"
	}

	return true, ""
}

func createJSONPatch(originalJSON, modifiedJSON []byte) ([]byte, error) {
	// Generate the JSON Patch
	patch, err := jsondiff.CompareJSON(originalJSON, modifiedJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create JSON Patch: %v", err)
	}

	// Marshal the patch operations to JSON bytes
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON Patch: %v", err)
	}

	return patchBytes, nil
}

func toAdmissionResponse(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

var getCRD = func(cr *unstructured.Unstructured) (*apiextensionsv1.CustomResourceDefinition, error) {
	// Build the client configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// Create the API extensions client
	apiExtensionsClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Create the discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}

	// Get the GroupVersionKind (GVK) of the resource
	gvk := cr.GroupVersionKind()

	// Get the preferred API resources
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %v", err)
	}

	// Build the REST mapper
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Map the GVK to a REST mapping
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to map GVK: %v", err)
	}

	// The name of the CRD is the plural form of the resource plus the group name
	plural := mapping.Resource.Resource
	crdName := fmt.Sprintf("%s.%s", plural, gvk.Group)

	// Get the CRD
	crd, err := apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve CRD: %v", err)
	}

	return crd, nil
}
