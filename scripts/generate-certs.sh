#!/bin/bash

# scripts/generate-certs.sh

# Set variables
NAMESPACE="default"
SERVICE_NAME="webhook-service"
SECRET_NAME="webhook-certs"
CERTS_DIR="config/certs"
TMP_DIR=$(mktemp -d)

# Ensure the certs directory exists
mkdir -p ${CERTS_DIR}

# Create the OpenSSL config for generating the certificate with SANs
cat <<EOF >> ${TMP_DIR}/csr.conf
[ req ]
default_bits       = 2048
distinguished_name = req_distinguished_name
req_extensions     = req_ext
prompt             = no

[ req_distinguished_name ]
CN = ${SERVICE_NAME}.${NAMESPACE}.svc

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.2 = ${SERVICE_NAME}
EOF

# Generate private key
openssl genrsa -out ${CERTS_DIR}/tls.key 2048

# Generate CSR using the configuration with SANs
openssl req -new -key ${CERTS_DIR}/tls.key -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" -out ${TMP_DIR}/tls.csr -config ${TMP_DIR}/csr.conf

# Self-sign the certificate with SANs
openssl x509 -req -in ${TMP_DIR}/tls.csr -signkey ${CERTS_DIR}/tls.key -out ${CERTS_DIR}/tls.crt -days 365 -extensions req_ext -extfile ${TMP_DIR}/csr.conf

# Create the Kubernetes secret for the webhook certificates
kubectl create secret generic ${SECRET_NAME} \
  --from-file=tls.crt=${CERTS_DIR}/tls.crt \
  --from-file=tls.key=${CERTS_DIR}/tls.key \
  --dry-run=client -o yaml > ${CERTS_DIR}/webhook-certs.yaml

echo "Generated TLS certificates and secret."
