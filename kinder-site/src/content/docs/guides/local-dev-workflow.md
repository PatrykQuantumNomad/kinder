---
title: Local Dev Workflow
description: Code, build, push to localhost:5001, deploy, and iterate with the local registry.
---

In this tutorial you will learn the complete code-build-push-deploy-iterate loop using kinder's built-in local registry at `localhost:5001`. No external registry, no authentication, no push delays — just a plain Docker push that the cluster can immediately pull from. You will build a Go HTTP server, deploy it, access it via port-forward, change the code, push a new version, and verify the update — all without leaving your local machine.

## Prerequisites

- [kinder installed](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH
- A text editor

## Step 1: Create the cluster

```sh
kinder create cluster
```

:::tip
The local registry is enabled by default and is accessible at `localhost:5001` as soon as the cluster is ready. You do not need to configure anything.
:::

## Step 2: Verify the registry is running

Confirm that the registry container is up:

```sh
docker ps --filter name=kind-registry
```

Expected output:

```
CONTAINER ID   IMAGE        COMMAND                  CREATED         STATUS         PORTS                    NAMES
a3f7b2c91d4e   registry:2   "/entrypoint.sh /etc…"   2 minutes ago   Up 2 minutes   0.0.0.0:5001->5000/tcp   kind-registry
```

The `kind-registry` container listens on `localhost:5001` on your host. Any image you push to it is immediately available to all nodes in the cluster.

## Step 3: Create a simple application

Create a new directory for the project and add two files:

```sh
mkdir myapp && cd myapp
```

**`main.go`:**

```go
package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from v1!")
	})
	http.ListenAndServe(":8080", nil)
}
```

**`Dockerfile`:**

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go build -o server main.go

FROM alpine:latest
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
```

:::tip
This Go server is just an example. The same workflow works with any language or framework — Node.js, Python, Rust, Java, or anything else that can be containerized. The key ingredients are Docker and `localhost:5001`.
:::

## Step 4: Build and push v1

Build the image and tag it with the local registry address:

```sh
docker build -t localhost:5001/myapp:v1 .
```

Push it to the local registry:

```sh
docker push localhost:5001/myapp:v1
```

Expected output:

```
The push refers to repository [localhost:5001/myapp]
a3d2f891b7c4: Pushed
e1f4a9c23d85: Pushed
v1: digest: sha256:4b2f8a1c9d3e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a size: 740
```

The image is now stored in `kind-registry` and is available for the cluster to pull.

## Step 5: Deploy to the cluster

Create a Deployment and a Service that reference the local registry image:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: myapp
          image: localhost:5001/myapp:v1
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
  ports:
    - port: 80
      targetPort: 8080
```

Save this as `deployment.yaml` and apply it:

```sh
kubectl apply -f deployment.yaml
```

Wait for the pod to be ready:

```sh
kubectl get pods
```

Expected output:

```
NAME                     READY   STATUS    RESTARTS   AGE
myapp-6d4f8b9c7-xk2pq   1/1     Running   0          20s
```

## Step 6: Access the application

Forward a local port to the `myapp` service:

```sh
kubectl port-forward svc/myapp 8080:80
```

In a separate terminal, make a request:

```sh
curl http://localhost:8080
```

Expected output:

```
Hello from v1!
```

:::note
`kubectl port-forward` works on Linux, macOS, and Windows. Press Ctrl+C in the port-forward terminal when you are done. You will need to restart it after each `kubectl rollout`.
:::

## Step 7: Make a change and iterate

Stop the port-forward with Ctrl+C, then edit `main.go` to change `"Hello from v1!"` to `"Hello from v2!"`.

After saving the file, run the full iteration loop:

```sh
docker build -t localhost:5001/myapp:v2 .
docker push localhost:5001/myapp:v2
kubectl set image deployment/myapp myapp=localhost:5001/myapp:v2
kubectl rollout status deployment/myapp
```

Expected output from `kubectl rollout status`:

```
Waiting for deployment "myapp" rollout to finish: 1 old replicas are pending termination...
deployment "myapp" successfully rolled out
```

:::tip[Iteration pattern]
The loop for every code change is:

1. Edit code
2. `docker build -t localhost:5001/myapp:vN .`
3. `docker push localhost:5001/myapp:vN`
4. `kubectl set image deployment/myapp myapp=localhost:5001/myapp:vN`
5. `kubectl rollout status deployment/myapp`

Use a new tag for each iteration (`:v2`, `:v3`, etc.) so Kubernetes always pulls the latest image. Reusing the same tag can cause the cluster to serve a cached layer.
:::

## Step 8: Verify the new version

Start port-forward again:

```sh
kubectl port-forward svc/myapp 8080:80
```

In a separate terminal:

```sh
curl http://localhost:8080
```

Expected output:

```
Hello from v2!
```

The rolling update replaced the old pod with one running the new image, with zero downtime.

## Alternative: skip the registry and use `kinder load images`

If you don't want to run a registry at all — or you're working in an air-gapped environment — kinder ships a `kinder load images` subcommand that bypasses the registry entirely. Instead of pushing to `localhost:5001`, you build the image locally and stream it directly into every node in the cluster.

Create the cluster with the local registry disabled if you prefer a leaner footprint:

```yaml
# cluster.yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  localRegistry: false
```

```sh
kinder create cluster --config cluster.yaml
```

Then the iteration loop becomes:

```sh
docker build -t myapp:dev .
kinder load images myapp:dev
kubectl rollout restart deployment/myapp
```

Set `imagePullPolicy: IfNotPresent` on your deployment so Kubernetes uses the image already loaded into the node instead of trying to pull it from a remote registry:

```yaml
containers:
  - name: myapp
    image: myapp:dev
    imagePullPolicy: IfNotPresent
```

:::tip[When to use each approach]
**Local Registry (`localhost:5001`)** — best for teams sharing a cluster, multiple images, or when you want a long-running registry you can push to from anywhere.

**`kinder load images`** — best for single-developer dev loops, air-gapped environments, or when you prefer a registry-free footprint. Smart-load skips re-import if the image is already present on all nodes, and the command works identically with docker, podman, nerdctl, finch, and nerdctl.lima.
:::

`kinder load images` also handles Docker Desktop 27+ containerd image store compatibility automatically via a two-attempt `ctr import` fallback. See the [Load Images CLI reference](/cli-reference/load-images/) for details on flags, provider behavior, and smart-load semantics.

## Clean up

```sh
kinder delete cluster
```

Optionally remove the project directory:

```sh
cd .. && rm -rf myapp
```

This removes the cluster and the local registry container. The `localhost:5001/myapp` images in the registry are gone with it.
