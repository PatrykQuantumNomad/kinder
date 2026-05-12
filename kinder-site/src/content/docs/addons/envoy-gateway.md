---
title: Envoy Gateway
description: Envoy Gateway addon for Kubernetes Gateway API routing in kinder clusters — pre-configured GatewayClass, server-side apply for large CRDs, and enabled by default.
---

Envoy Gateway brings the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/) to kinder clusters. It replaces Ingress with a more expressive routing model and supports HTTP, TLS, and TCP routes with a single controller.

kinder installs Envoy Gateway **v1.7.2**.

:::caution[Major upgrade in v2.4 — Envoy Gateway v1.3 → v1.7]
This is a two-major-version jump. Bundled Gateway API CRDs jump from `v1.2.1` to `v1.4.1`.

- **Existing kinder clusters:** unaffected — addon versions are pinned at cluster-create time. Recreate the cluster to pick up v1.7.2.
- **HTTPRoute compatibility:** the v1.4.1 Gateway API CRD bundle is backwards compatible with v1.4.x and most v1.2.x HTTPRoute manifests. Custom resources using deprecated v1alpha2 fields may need updates — see [upstream Gateway API release notes](https://gateway-api.sigs.k8s.io/release-notes/).
- **No Helm migration needed:** kinder uses upstream's `install.yaml` directly via `kubectl apply`; the change is fully manifest-driven.
:::

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

## Practical examples

### Path-based routing

Route `/api` requests to one service and `/web` requests to another using a single HTTPRoute:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: path-route
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api
      backendRefs:
        - name: api-service
          port: 8080
    - matches:
        - path:
            type: PathPrefix
            value: /web
      backendRefs:
        - name: web-service
          port: 80
```

:::note
`PathPrefix` matches any path that starts with the given value — `/api` matches `/api/users` and `/api/v2/items`. Use `Exact` if you want to match only the literal path `/api` with nothing following it.
:::

### Header-based routing

Route requests to a canary deployment based on a request header:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: header-routing
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - headers:
            - name: X-Environment
              value: canary
      backendRefs:
        - name: canary-service
          port: 80
    - backendRefs:
        - name: stable-service
          port: 80
```

Test canary routing with curl:

```sh
# Route to canary
curl -H "X-Environment: canary" http://<gateway-ip>/

# Route to stable (no header)
curl http://<gateway-ip>/
```

## Troubleshooting

### Gateway stuck in Pending

**Symptom:** `kubectl get gateway` shows the gateway status is not `Programmed` — it remains `Pending` or shows no `PROGRAMMED` column value.

**Cause:** Either the Envoy Gateway controller is not running, or MetalLB is unavailable to assign an external IP to the gateway's backing service.

**Fix:**

Check that the Envoy Gateway controller pod is running:

```sh
kubectl get pods -n envoy-gateway-system
```

If the pod is not `Running`, describe it for error details.

Verify that MetalLB is enabled and its pods are healthy:

```sh
kubectl get pods -n metallb-system
```

If MetalLB is disabled or unhealthy, the gateway's LoadBalancer service will stay in `<pending>` and the gateway will not become `Programmed`.

## Technical note

:::note[Server-side apply]
kinder installs Envoy Gateway using **server-side apply** (`kubectl apply --server-side`). This is required because the Envoy Gateway CRDs are larger than 256 KB after annotation — they exceed the client-side apply annotation limit imposed by `kubectl`.

Server-side apply offloads the field tracking to the API server, which removes this constraint. No action is needed on your part; kinder handles this automatically.
:::
