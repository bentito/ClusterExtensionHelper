# Makefile for building the webhook project

BINARY_NAME := webhook
OUTPUT_DIR := bin
IMAGE_NAME := webhook:latest
NAMESPACE := default
CLUSTER_NAME ?= operator-controller

SCRIPTS_DIR := scripts
CONFIG_DIR := config
CERTS_DIR := $(CONFIG_DIR)/certs

.PHONY: all build deps test clean make-cert deploy undeploy deploy-openai deploy-local-llm

all: build

build: deps
	mkdir -p $(OUTPUT_DIR)
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME) cmd/main.go

deps:
	go mod tidy

test:
	go test -v ./...

clean:
	rm -rf $(OUTPUT_DIR)

make-cert:
	@echo "Generating TLS certificates..."
	./$(SCRIPTS_DIR)/generate-certs.sh

# Default deploy target: OpenAI
deploy: deploy-openai

# Deploy with OpenAI API
deploy-openai: build make-cert
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME) .
	@echo "Loading Docker image into Kind cluster..."
	kind load docker-image $(IMAGE_NAME) --name ${CLUSTER_NAME}
	@echo "Generating OpenAI API Key secret..."
	./$(SCRIPTS_DIR)/generate-openai-secret.sh
	@echo "Applying Kubernetes manifests for OpenAI..."
	kubectl apply -f $(CERTS_DIR)/webhook-certs.yaml
	kubectl apply -f $(CONFIG_DIR)/openai-api-key.yaml
	kubectl apply -f $(CONFIG_DIR)/deployment.yaml
	kubectl apply -f $(CONFIG_DIR)/service.yaml
	@echo "Configuring MutatingWebhookConfiguration for OpenAI..."
	./$(SCRIPTS_DIR)/generate-mutatingwebhookconfiguration.sh
	kubectl apply -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	kubectl apply -f config/rbac/serviceaccount.yaml
	kubectl apply -f config/rbac/clusterrole.yaml
	kubectl apply -f config/rbac/clusterrolebinding.yaml
	rm -f ca.crt
	@echo "Deployment with OpenAI complete."

# Deploy with Local LLM
deploy-local-llm: build make-cert
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME) .
	@echo "Loading Docker image into Kind cluster..."
	kind load docker-image $(IMAGE_NAME) --name ${CLUSTER_NAME}
	@echo "Applying Kubernetes manifests for Local LLM..."
	kubectl apply -f $(CERTS_DIR)/webhook-certs.yaml
	kubectl apply -f $(CONFIG_DIR)/deployment-llm.yaml
	kubectl apply -f $(CONFIG_DIR)/service.yaml
	@echo "Configuring MutatingWebhookConfiguration for Local LLM..."
	./$(SCRIPTS_DIR)/generate-mutatingwebhookconfiguration.sh
	kubectl apply -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	kubectl apply -f config/rbac/serviceaccount.yaml
	kubectl apply -f config/rbac/clusterrole.yaml
	kubectl apply -f config/rbac/clusterrolebinding.yaml
	rm -f ca.crt
	@echo "Deployment with Local LLM complete."

undeploy:
	@echo "Deleting Kubernetes resources..."
	-kubectl delete -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	-kubectl delete -f $(CONFIG_DIR)/deployment.yaml
	-kubectl delete -f $(CONFIG_DIR)/deployment-llm.yaml
	-kubectl delete -f $(CONFIG_DIR)/service.yaml
	-kubectl delete secret webhook-certs
	-kubectl delete secret openai-api-key
	rm -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	rm -f $(CONFIG_DIR)/openai-api-key.yaml
	rm -f $(CERTS_DIR)/webhook-certs.yaml
	@echo "Undeployment complete."
