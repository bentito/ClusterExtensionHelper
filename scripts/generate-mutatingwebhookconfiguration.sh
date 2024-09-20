#!/bin/bash

# scripts/generate-mutatingwebhookconfiguration.sh

set -e

CONFIG_DIR=config

# Extract the CA certificate from the secret
CA_BUNDLE=$(kubectl get secret webhook-certs -o jsonpath='{.data.tls\.crt}')

# Export the CA_BUNDLE variable for envsubst
export CA_BUNDLE="${CA_BUNDLE}"

# Use envsubst to substitute the placeholder in the template
envsubst < "${CONFIG_DIR}/mutatingwebhookconfiguration.yaml.template" > "${CONFIG_DIR}/mutatingwebhookconfiguration.yaml"

echo "Generated ${CONFIG_DIR}/mutatingwebhookconfiguration.yaml with updated CA_BUNDLE."
