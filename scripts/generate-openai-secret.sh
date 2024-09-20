#!/bin/bash

CONFIG_DIR=config

# generate-openai-secret.sh

if [ -z "$OPENAI_API_KEY" ]; then
  echo "Error: OPENAI_API_KEY environment variable is not set."
  exit 1
fi

cat <<EOF > ${CONFIG_DIR}/openai-api-key.yaml
apiVersion: v1
kind: Secret
metadata:
  name: openai-api-key
type: Opaque
stringData:
  api-key: "$OPENAI_API_KEY"
EOF

echo "openai-api-key.yaml has been created."

