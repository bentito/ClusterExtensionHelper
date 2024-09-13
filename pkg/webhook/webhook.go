package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"gopkg.in/yaml.v2"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
)

var (
	scheme       = runtime.NewScheme()
	codecs       = serializer.NewCodecFactory(scheme)
	deserializer = codecs.UniversalDeserializer()
)

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

	// Process the AdmissionRequest
	admissionResponse := mutate(&admissionReview)

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

func getCRD(cr *unstructured.Unstructured) (*apiextensionsv1.CustomResourceDefinition, error) {
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

	// Construct the CRD name
	// CRD names are in the format <plural>.<group>
	group := cr.GroupVersionKind().Group
	if err != nil {
		return nil, err
	}

	crdName := fmt.Sprintf("%s.%s", cr.GetKind(), group)

	// Get the CRD
	crd, err := apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return crd, nil
}

func mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
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
	adjustedCR, err := AdjustCRWithLLM(cr, crd)
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

func AdjustCRWithLLM(cr *unstructured.Unstructured, crd *apiextensionsv1.CustomResourceDefinition) (*unstructured.Unstructured, error) {
	// Convert CR to YAML
	crYAML, err := yaml.Marshal(cr.Object)
	if err != nil {
		return nil, err
	}

	// Convert CRD to YAML
	crdYAML, err := yaml.Marshal(crd)
	if err != nil {
		return nil, err
	}

	// Construct the prompt
	prompt := fmt.Sprintf(`You are an expert in Kubernetes custom resources. Given the following Custom Resource Definition (CRD):

---
%s
---

And the following Custom Resource (CR) that may not conform to the CRD:

---
%s
---

Adjust the CR so that it conforms to the CRD schema. Return only the corrected CR in YAML format. Do not include any explanations or additional text.`, string(crdYAML), string(crYAML))

	// Initialize the OpenAI client
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}
	client := openai.NewClient(
		option.WithAPIKey(openaiAPIKey),
	)

	// Create the ChatCompletion request
	ctx := context.TODO()
	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		}),
		Model: openai.F("gpt-4"), // Use "gpt-3.5-turbo" if "gpt-4" is not accessible
	})
	if err != nil {
		return nil, err
	}

	if len(chatCompletion.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	adjustedCRYAML := chatCompletion.Choices[0].Message.Content

	// Convert YAML back to unstructured.Unstructured
	adjustedCR := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(adjustedCRYAML), &adjustedCR.Object)
	if err != nil {
		return nil, err
	}

	return adjustedCR, nil
}

func ValidateCR(cr *unstructured.Unstructured) (bool, string) {
	// Implement your validation logic here
	// For demonstration, let's assume the CR must have 'spec' with 'requiredField'
	spec, found, err := unstructured.NestedMap(cr.Object, "spec")
	if err != nil || !found {
		return false, "spec field is missing"
	}

	if _, exists := spec["requiredField"]; !exists {
		return false, "requiredField is missing in spec"
	}

	return true, ""
}

func createJSONPatch(originalJSON, modifiedJSON []byte) ([]byte, error) {
	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(originalJSON, modifiedJSON, originalJSON)
	if err != nil {
		return nil, err
	}
	return patch, nil
}

func toAdmissionResponse(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}
