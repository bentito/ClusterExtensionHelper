### Using the ai-lab-recipes repo

- Install podman
- In `ai-lab-recipes/recipes/natural_language_processing/chatbot/` find `build` dir with configs
- Below is a sample config that works:
```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: chatbot
  name: chatbot
spec:
  initContainers:
  - name: model-file
    image: quay.io/ai-lab/granite-7b-lab:latest
    command: ['/usr/bin/install', "/model/model.file", "/shared/"]
    volumeMounts:
    - name: model-file
      mountPath: /shared
  containers:
  - name: chatbot-inference
    image: quay.io/ai-lab/chatbot:latest
    env:
    - name: MODEL_ENDPOINT
      value: http://localhost:8001/v1/chat/completions
    ports:
    - containerPort: 8501
      hostPort: 8501  # Ensure port is bound to the host
    securityContext:
      runAsNonRoot: true
  - name: chatbot-model-service
    image: quay.io/ai-lab/llamacpp_python:latest
    env:
    - name: HOST
      value: 0.0.0.0
    - name: PORT
      value: 8001
    - name: MODEL_PATH
      value: /model/model.file
    ports:
    - containerPort: 8001
      hostPort: 8001  # Ensure port is bound to the host
    securityContext:
      runAsNonRoot: true
    volumeMounts:
    - name: model-file
      mountPath: /model
  volumes:
  - name: model-file
    emptyDir: {}
```

> Note: this is only allows a 2K context window.

- start with: `podman kube play build/2k-working-chatbot.yaml`
- restart with: `podman pod start chatbot`
- stop with: `podman pod stop chatbot`
- clear out with: `podman pod rm chatbot`
- check running with: `podman pod ps`

An alternate with larger context window (16k):

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: chatbot
  name: chatbot
spec:
  containers:
    - name: chatbot-inference
      image: quay.io/ai-lab/chatbot:latest
      env:
        - name: MODEL_ENDPOINT
          value: http://localhost:8001/v1/chat/completions
        - name: MAX_CONTEXT_TOKENS
          value: "16384"
      ports:
        - containerPort: 8501
          hostPort: 8501
      securityContext:
        runAsNonRoot: true

    - name: chatbot-model-service
      image: ghcr.io/huggingface/text-generation-inference:latest
      command:
        - text-generation-launcher
        - "--model-id"
        - "mistralai/Mistral-7B-Instruct-v0.1"
        - "--max-input-length"
        - "16383"
        - "--max-total-tokens"
        - "16384"
        - "--trust-remote-code"
        - "--port"
        - "8001"
      env:
        - name: HF_HUB_ENABLE_HF_TRANSFER
          value: "true"
        - name: TEXT_GENERATION_API
          value: "openai"
        - name: HF_API_TOKEN
          value: "<snip>"
      ports:
        - containerPort: 8001
          hostPort: 8001
      securityContext:
        runAsNonRoot: true
```

Alternatively run local LLM this way:

```bash
brew install ollama
export OLLAMA_HOST=localhost:8001
ollama serve
ollama pull granite-code:8b
```
or
```bash
ollama pull granite-code:3b-instruct-128k-fp16
ollama show granite-code:3b-instruct-128k-fp16  --modelfile > granite-code:3b-instruct-128k-fp16.modelfile
vi granite-code:3b-instruct-128k-fp16.modelfile
```
add `PARAMETER num_ctx 16384`
```bash
ollama create -f granite-code:3b-instruct-128k-fp16.modelfile granite-code:3b-instruct-128k-fp16
```
which overwrites the model being served with one with actual expanded context window

A very slow what to get latest `granite3-8b-instruct` serving (this is the latest InstructLAB enhanced model): 
```bash
podman run --rm -it -p 8080:8080 quay.io/redhat-user-workloads/ilab-community-tenant/granite-3-8b-instruct:97638ed506816841c5ea22e9c153ff8fadf96806-linux-arm64 --serve
```
It does work well with our PoC prompt, filling in the `packageName`