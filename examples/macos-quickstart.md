
# MacOS Quickstart for Prompt Engineering with Ollama and Curl

## Prerequisites

1. **Install Ollama**:
   ```bash
   brew install ollama
   ```
2. **Set the Ollama Host** (useful if running on `localhost`):
   ```bash
   export OLLAMA_HOST=localhost:8001
   ```

## Setting Up the Model Server

1. **Serve the Model**:
   ```bash
   ollama serve &
   ```
2. **Pull a Model**:
   - For `granite-code` model:
     ```bash
     ollama pull granite-code:8b
     ```
   - Alternatively, for a version with a larger context window:
     ```bash
     ollama pull granite-code:3b-instruct-128k-fp16
     ollama show granite-code:3b-instruct-128k-fp16 --modelfile > granite-code:3b-instruct-128k-fp16.modelfile
     ```
     Edit the `.modelfile`:
     ```bash
     vi granite-code:3b-instruct-128k-fp16.modelfile
     ```
     Add:
     ```
     PARAMETER num_ctx 16384
     ```
     Then, create the model with expanded context:
     ```bash
     ollama create -f granite-code:3b-instruct-128k-fp16.modelfile granite-code:3b-instruct-128k-fp16
     ```

## Example Usage with Curl

To send prompts to the model using curl:

1. **Request with Mistral-Nemo model**:
   ```bash
   curl --location 'http://127.0.0.1:8001/v1/chat/completions'    --header 'Content-Type: application/json'    --data @examples/condensed_crd_mistral_nemo_prompt_example.json | jq '. | {id, model, created, system_fingerprint, content: (.choices[0].message.content | @text), usage}'
   ```

2. **Request with Granite Model**:
   ```bash
   curl --location 'http://127.0.0.1:8001/v1/chat/completions'    --header 'Content-Type: application/json'    --data @examples/condensed_crd_granite-code-3b-instruct-128k-fp16_prompt_example.json | jq '. | {id, model, created, system_fingerprint, content: (.choices[0].message.content | @text), usage}'
   ```

This setup provides a streamlined way to test prompt engineering locally on macOS using the Ollama model server and curl.
