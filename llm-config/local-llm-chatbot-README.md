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

