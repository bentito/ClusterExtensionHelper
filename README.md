
# Soft ClusterExtension Admission with LLM Integration

## Overview

This project implements a Kubernetes admission webhook that automatically corrects invalid ClusterExtension Custom Resources (CRs) using a Large Language Model (LLM). When a ClusterExtension CR fails validation against its Custom Resource Definition (CRD), the webhook intercepts the create or update request, leverages an LLM to adjust the CR, and returns a JSON Patch to correct the resource before it's persisted in the cluster.

By integrating an LLM, this webhook aims to simplify the management of CRs by automatically fixing common errors, reducing the manual effort required to maintain valid configurations.

## Watch the demos

https://youtu.be/XLyHyTMrcS4 (OpenAI succeeds at the task)

https://youtu.be/EEi7GEAK8v0 (An open source LLM succeeds at the task)

## Prerequisites

- **Podman (or Docker)**: For building and loading container images.
- **Kind**: To create a local Kubernetes cluster.
- **kubectl**: For interacting with your Kubernetes cluster.
- **OpenAI API Key**: Required to interact with the OpenAI API. (for now, this is the LLM)

## Usage

Follow the steps below to build, deploy, and test the admission webhook on a local Kind cluster.

### 1. Set Up Your Environment

Ensure you have the necessary tools installed:

- Install [Podman](https://podman.io/) or [Docker](https://docs.docker.com/get-docker/).
- Install [Kind](https://kind.sigs.k8s.io/).
- Install [kubectl](https://kubernetes.io/docs/tasks/tools/).

### 2. Create a Kind Cluster

Create a local Kubernetes cluster using Kind:

```bash
kind create cluster
```

### 3. Export Your OpenAI API Key

Set the `OPENAI_API_KEY` environment variable with your actual OpenAI API key:

```bash
export OPENAI_API_KEY=your_actual_openai_api_key
```

> **Note**: Replace `your_actual_openai_api_key` with your actual OpenAI API key. Keep this key secure and do not share it or commit it to version control.

### 3.1 Alternative: Using Your Own LLM

Alternatively, you can configure the webhook to use your own LLM that supports the completions API by setting the `LOCAL_LLM_URL` environment variable. This setup requires an alternate deployment configuration. Instead of using the default OpenAI deployment configuration (`config/deployment.yaml`), switch to `config/deployment-llm.yaml`.

```bash
export LOCAL_LLM_URL=http://your-llm-instance/api
```

This will allow you to process LLM prompts with your own hosted LLM.

### 4. Build and Deploy the Webhook

Use the provided Makefile to build the Go application, create Docker images, generate necessary certificates and secrets, and deploy the webhook to your Kind cluster.

Run the following command from the root of the project:

```bash
make deploy
```

This command performs the following actions:

- Builds the Go application and creates the webhook binary.
- Builds the Docker image and loads it into the Kind cluster.
- Generates TLS certificates for secure communication and creates the `webhook-certs.yaml` secret in `config/`.
- Generates the `openai-api-key.yaml` secret in `config/` using the `OPENAI_API_KEY` environment variable.
- Applies Kubernetes manifests from the `config/` directory to deploy the webhook and its service.
- Configures the `MutatingWebhookConfiguration` with the correct CA bundle to register the webhook with the Kubernetes API server.

### 5. Test the Webhook

#### 5.1 Create an Invalid ClusterExtension CR

Create a YAML file named `invalid-cr.yaml` with the following content:

```yaml
apiVersion: olm.operatorframework.io/v1alpha1
kind: ClusterExtension
metadata:
  name: test-extension
spec:
  install:
    namespace: test-namespace
    serviceAccount:
      name: test-sa
    source:
      sourceType: Catalog
      catalog:
        # packageName is missing
```

This CR is intentionally invalid because it is missing the required `packageName` field under `spec.source.catalog`.

#### 5.2 Apply the Invalid CR

Apply the invalid CR to your cluster:

```bash
kubectl apply -f invalid-cr.yaml --validate=false
```

#### 5.3 Verify the Mutation

Retrieve the CR to see if it has been mutated by the webhook:

```bash
kubectl get clusterextension test-extension -o yaml
```

Check if the `packageName` field has been added or corrected by the webhook.

#### 5.4 Observe the Webhook Logs

View the logs of the webhook pod to observe its behavior:

```bash
# Get the webhook pod name
POD_NAME=$(kubectl get pods -l app=webhook -o jsonpath='{.items[0].metadata.name}')

# View the logs
kubectl logs $POD_NAME
```

### 6. Undeploy the Webhook

When you're finished testing, you can remove all deployed resources using:

```bash
make undeploy
```

This command deletes the Kubernetes resources and cleans up generated files.

### Cleaning Up

To delete the Kind cluster entirely:

```bash
kind delete cluster
```

## Important Notes

- **Security**: Ensure that sensitive files like `openai-api-key.yaml`, `webhook-certs.yaml`, and `mutatingwebhookconfiguration.yaml` are not committed to version control. They are automatically ignored if you have the appropriate entries in your `.gitignore` file.
  
  Add the following lines to `.gitignore`:

  ```
  config/openai-api-key.yaml
  config/webhook-certs.yaml
  config/mutatingwebhookconfiguration.yaml
  ```

- **Executable Permissions**: Ensure that the scripts in the `scripts/` directory have executable permissions:

  ```bash
  chmod +x scripts/generate-certs.sh
  chmod +x scripts/generate-openai-secret.sh
  ```

- **Namespace Consistency**: The default namespace used is `default`. If you wish to deploy to a different namespace, update the `NAMESPACE` variable in the Makefile, scripts, and Kubernetes manifests accordingly.

- **OpenAI API Usage**: Be mindful of the OpenAI API usage limits and associated costs when testing the webhook, as each invalid CR will trigger a request to the OpenAI API.

## Project Details

### How It Works

1. **Admission Webhook Interception**: When a ClusterExtension CR is created or updated, the Kubernetes API server sends the request to the admission webhook for validation and possible mutation.
   
2. **Validation**: The webhook validates the CR against its CRD schema using the `ValidateCR` function.
   
3. **LLM Adjustment**: If the CR is invalid, the webhook calls the `AdjustCRWithLLM` function, which sends the CR and its validation errors to the OpenAI API. The LLM attempts to correct the CR based on the provided schema and errors.
   
4. **Patch Generation**: A JSON Patch is generated based on the differences between the original CR and the adjusted CR returned by the LLM.
   
5. **Response to API Server**: The webhook returns an admission response containing the JSON Patch, which the API server applies to the original CR before persisting it.

### Customization

- **CRD Schema**: The webhook relies on the ClusterExtension CRD schema. Ensure that the schema is accurate and up-to-date.
- **LLM Prompting**: The prompts sent to the OpenAI API can be customized within the webhook code to improve correction accuracy.
- **Error Handling**: Enhance error handling and logging in the webhook to handle different scenarios gracefully.

## Development

### Running Tests

To run the unit tests for the webhook:

```bash
make test
```

### Building the Binary

To build the webhook binary without deploying:

```bash
make build
```

The binary will be placed in the `bin/` directory.

## Contributing

Contributions are welcome! Please open issues or pull requests for enhancements, bug fixes, or other improvements.

## License

This project is licensed under the MIT License.
