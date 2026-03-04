---
title: TLS Web App
description: Deploy a TLS-secured web app with Local Registry, Envoy Gateway, cert-manager, and MetalLB.
---

In this tutorial you will deploy a TLS-secured nginx application on a local Kubernetes cluster. You will push a container image to the **Local Registry**, expose it through **Envoy Gateway** with HTTPS termination, use **cert-manager** to issue a self-signed TLS certificate, and rely on **MetalLB** to assign the gateway an external IP — all without any manual addon installation.

## Prerequisites

- kinder installed — see [Installation](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH
- `curl` installed (HTTPS verification uses the `-k` flag to accept the self-signed certificate)

## Step 1: Create the cluster

```sh
kinder create cluster
```

:::tip
All four addons used in this tutorial — Local Registry, Envoy Gateway, cert-manager, and MetalLB — are enabled by default. No `--profile` flag or extra configuration is needed.
:::

Expected output:

```
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.32.0) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
 ✓ Installing addons
    metallb            1.8s
    local-registry     0.3s
    envoy-gateway      4.2s
    cert-manager      38.1s
Set kubectl context to "kind-kind"
```

## Step 2: Verify addons are ready

Check that all cert-manager components are running:

```sh
kubectl get pods -n cert-manager
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS   AGE
cert-manager-...                           1/1     Running   0          60s
cert-manager-cainjector-...                1/1     Running   0          60s
cert-manager-webhook-...                   1/1     Running   0          60s
```

:::note
cert-manager's webhook takes 30–60 seconds to become ready after cluster creation. If you apply Certificate resources before all three pods are `Running`, the webhook may reject them. Wait until all three show `Running` before continuing.
:::

Check that the Envoy Gateway controller is running:

```sh
kubectl get pods -n envoy-gateway-system
```

Expected output:

```
NAME                                           READY   STATUS    RESTARTS   AGE
envoy-gateway-...                              1/1     Running   0          60s
```

Check that MetalLB is running:

```sh
kubectl get pods -n metallb-system
```

Expected output:

```
NAME                          READY   STATUS    RESTARTS   AGE
controller-...                1/1     Running   0          60s
speaker-...                   1/1     Running   0          60s
```

Verify the local registry container is running:

```sh
docker ps --filter name=kind-registry
```

Expected output:

```
CONTAINER ID   IMAGE        ...   PORTS                    NAMES
abc123         registry:2   ...   0.0.0.0:5001->5000/tcp   kind-registry
```

## Step 3: Push an image to the local registry

Pull the nginx Alpine image, tag it for the local registry, and push it:

```sh
docker pull nginx:alpine
docker tag nginx:alpine localhost:5001/myapp:v1
docker push localhost:5001/myapp:v1
```

Expected output:

```
v1: digest: sha256:a1b2c3d4e5f6... size: 8192
```

## Step 4: Deploy the application

Apply the Deployment and Service in one step:

```sh
kubectl apply -f - <<EOF
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
            - containerPort: 80
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
      targetPort: 80
EOF
```

Verify the pod is running:

```sh
kubectl get pods
```

Expected output:

```
NAME                     READY   STATUS    RESTARTS   AGE
myapp-...                1/1     Running   0          15s
```

## Step 5: Create a TLS certificate

Apply a Certificate resource that cert-manager will fulfill using the pre-installed `selfsigned-issuer`:

```sh
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: myapp-tls
  namespace: default
spec:
  secretName: myapp-tls
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  dnsNames:
    - myapp.local
EOF
```

:::caution
Use `kind: ClusterIssuer`, not `kind: Issuer`. The kinder-provisioned issuer is a ClusterIssuer. Using `Issuer` will cause the certificate to stay `READY: False`.
:::

Wait for the certificate to become ready:

```sh
kubectl get certificate myapp-tls
```

Expected output:

```
NAME        READY   SECRET      AGE
myapp-tls   True    myapp-tls   15s
```

## Step 6: Create the TLS gateway

Apply a Gateway that terminates TLS using the certificate secret:

```sh
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: myapp-gateway
spec:
  gatewayClassName: eg
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: myapp-tls
EOF
```

Wait for MetalLB to assign an external IP and for the gateway to become programmed:

```sh
kubectl get gateway myapp-gateway
```

Expected output:

```
NAME             CLASS   ADDRESS          PROGRAMMED   AGE
myapp-gateway    eg      172.20.255.200   True         20s
```

## Step 7: Create an HTTPRoute

Route incoming HTTPS traffic to the `myapp` service:

```sh
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-route
spec:
  parentRefs:
    - name: myapp-gateway
  rules:
    - backendRefs:
        - name: myapp
          port: 80
EOF
```

## Step 8: Verify HTTPS access

Get the gateway's external IP and send an HTTPS request:

```sh
GATEWAY_IP=$(kubectl get gateway myapp-gateway -o jsonpath='{.status.addresses[0].value}')
curl -k https://$GATEWAY_IP
```

Expected output:

```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
...
</html>
```

The `-k` flag tells curl to accept the self-signed certificate.

:::note[macOS / Windows]
On macOS and Windows, LoadBalancer IPs are assigned inside the Docker or Podman VM network and are not directly reachable from the host. Use port-forward instead.

First, discover the Envoy Gateway service name:

```sh
kubectl get svc -n envoy-gateway-system
```

Expected output:

```
NAME                                    TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)
envoy-default-myapp-gateway-<hash>      LoadBalancer   10.96.10.100   172.20.255.200   443:31443/TCP
```

Forward the gateway port to localhost using the service name from the output above:

```sh
kubectl port-forward svc/envoy-default-myapp-gateway-<hash> 8443:443 -n envoy-gateway-system
```

Then in a separate terminal:

```sh
curl -k https://localhost:8443
```

Expected output:

```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
...
</html>
```
:::

## Clean up

Delete the cluster when you are done:

```sh
kinder delete cluster
```
