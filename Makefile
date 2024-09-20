# Makefile for building the webhook project

BINARY_NAME := webhook
OUTPUT_DIR := bin
IMAGE_NAME := webhook:latest
NAMESPACE := default
CLUSTER_NAME ?= operator-controller

SCRIPTS_DIR := scripts
CONFIG_DIR := config
CERTS_DIR := $(CONFIG_DIR)/certs

.PHONY: all build deps test clean make-cert deploy undeploy

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

deploy: build make-cert
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME) .
	@echo "Loading Docker image into Kind cluster..."
	kind load docker-image $(IMAGE_NAME) --name ${CLUSTER_NAME}
	@echo "Generating OpenAI API Key secret..."
	./$(SCRIPTS_DIR)/generate-openai-secret.sh
	@echo "Applying Kubernetes manifests..."
	kubectl apply -f $(CERTS_DIR)/webhook-certs.yaml
	kubectl apply -f $(CONFIG_DIR)/openai-api-key.yaml
	kubectl apply -f $(CONFIG_DIR)/deployment.yaml
	kubectl apply -f $(CONFIG_DIR)/service.yaml
	@echo "Configuring MutatingWebhookConfiguration..."
	./$(SCRIPTS_DIR)/generate-mutatingwebhookconfiguration.sh
	@echo "Applying MutatingWebhookConfiguration..."
	kubectl apply -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	@echo "Applying RBAC and ServiceAccount..."
	kubectl apply -f config/rbac/serviceaccount.yaml
	kubectl apply -f config/rbac/clusterrole.yaml
	kubectl apply -f config/rbac/clusterrolebinding.yaml
	rm -f ca.crt
	@echo "Deployment complete."

undeploy:
	@echo "Deleting Kubernetes resources..."
	-kubectl delete -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	-kubectl delete -f $(CONFIG_DIR)/deployment.yaml
	-kubectl delete -f $(CONFIG_DIR)/service.yaml
	-kubectl delete secret webhook-certs
	-kubectl delete secret openai-api-key
	rm -f $(CONFIG_DIR)/mutatingwebhookconfiguration.yaml
	rm -f $(CONFIG_DIR)/openai-api-key.yaml
	rm -f $(CERTS_DIR)/webhook-certs.yaml
	@echo "Undeployment complete."
