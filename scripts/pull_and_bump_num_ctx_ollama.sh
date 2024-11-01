#!/bin/bash

# Check if model name is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <model-name>"
  exit 1
fi

MODEL_NAME=$1
MODFILE="${MODEL_NAME}.modelfile"

# Step 1: Pull the model
ollama pull "$MODEL_NAME"

# Step 2: Save the model file details
ollama show "$MODEL_NAME" --modelfile > "$MODFILE"

# Step 3: Insert the PARAMETER line before LICENSE
sed -i '/LICENSE """/i PARAMETER num_ctx 16384' "$MODFILE"

# Step 4: Create the new model using modified modelfile
ollama create -f "$MODFILE" "$MODEL_NAME"
