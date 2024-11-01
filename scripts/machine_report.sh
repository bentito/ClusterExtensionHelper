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
mail_reknew_gmail_com@gpu-ollama-server:~$ cat machine-report.sh
#!/bin/bash

echo "=== System Specifications ==="

# CPU Information
echo -e "\n** CPU Info **"
lscpu | grep -E 'Model name|Socket|Core|Thread|MHz'

# Memory (RAM)
echo -e "\n** Memory (RAM) **"
free -h | awk '/^Mem:/ {print "Total:", $2, "\nUsed:", $3, "\nFree:", $4}'

# GPU Information
echo -e "\n** GPU Info **"
if command -v nvidia-smi &> /dev/null; then
    nvidia-smi --query-gpu=name,memory.total --format=csv,noheader
else
    lspci | grep -i vga
    echo "(If NVIDIA GPU is present, install 'nvidia-smi' for more details)"
fi

# Disk Space
echo -e "\n** Disk Space **"
df -h --total | awk '/^total/ {print "Total:", $2, "\nUsed:", $3, "\nFree:", $4}'

echo -e "\n=== End of Report ==="
