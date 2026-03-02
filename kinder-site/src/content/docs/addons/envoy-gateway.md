---
title: Envoy Gateway
description: Envoy Gateway addon for Gateway API routing in kinder clusters.
---

Envoy Gateway brings the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/) to kinder clusters. It replaces Ingress with a more expressive routing model and supports HTTP, TLS, and TCP routes with a single controller.

kinder installs Envoy Gateway **v1.3.1**.

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| Envoy Gateway controller | `envoy-gateway-system` | Watches Gateway/HTTPRoute resources |
| `GatewayClass` "eg" | cluster-scoped | Entry point for all Gateway resources |
| Gateway API CRDs | cluster-scoped | `Gateway`, `HTTPRoute`, `GRPCRoute`, etc. |

## How to use

Create a `Gateway` and an `HTTPRoute` using the `eg` GatewayClass:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
spec:
  gatewayClassName: eg
  listeners:
    - name: http
      protocol: HTTP
      port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: my-service
          port: 80
```

MetalLB assigns the gateway an external IP automatically when both addons are enabled.

## How to verify

Check that the GatewayClass is accepted:

```sh
kubectl get gatewayclass eg
```

Expected output:

```
NAME   CONTROLLER                        ACCEPTED   AGE
eg     gateway.envoyproxy.io/gatewayclass   True       60s
```

Verify the controller pod is running:

```sh
kubectl get pods -n envoy-gateway-system
```

## Configuration

Envoy Gateway is controlled by the `addons.envoyGateway` field:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  envoyGateway: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  envoyGateway: false
```

## Technical note

:::note[Server-side apply]
kinder installs Envoy Gateway using **server-side apply** (`kubectl apply --server-side`). This is required because the Envoy Gateway CRDs are larger than 256 KB after annotation — they exceed the client-side apply annotation limit imposed by `kubectl`.

Server-side apply offloads the field tracking to the API server, which removes this constraint. No action is needed on your part; kinder handles this automatically.
:::
