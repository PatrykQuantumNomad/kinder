---
title: cert-manager
description: cert-manager addon for automatic TLS certificate management in kinder clusters.
---

cert-manager gives kinder clusters automatic TLS certificate management. A self-signed `ClusterIssuer` is created during cluster setup so you can issue `Certificate` resources immediately — no manual cert-manager installation or issuer configuration required.

kinder installs cert-manager **v1.16.3**.

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| cert-manager controller | `cert-manager` | Watches Certificate resources, issues certs |
| cert-manager-cainjector | `cert-manager` | Injects CA bundles into webhooks and API services |
| cert-manager-webhook | `cert-manager` | Validates and converts cert-manager resources |
| `selfsigned-issuer` ClusterIssuer | cluster-scoped | Self-signed issuer ready for immediate use |

## How to use

Create a self-signed certificate:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-cert
  namespace: default
spec:
  secretName: my-cert-tls
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  dnsNames:
    - myapp.local
```

Apply it and verify:

```sh
kubectl apply -f certificate.yaml
kubectl get certificate my-cert
```

Expected output:

```
NAME      READY   SECRET        AGE
my-cert   True    my-cert-tls   10s
```

The TLS secret is now available for use in Ingress, Gateway, or pod volume mounts:

```sh
kubectl get secret my-cert-tls -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -subject
```

### Use with Envoy Gateway

cert-manager pairs naturally with the Envoy Gateway addon. Create a Gateway with TLS termination:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  annotations:
    cert-manager.io/cluster-issuer: selfsigned-issuer
spec:
  gatewayClassName: eg
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: my-gateway-tls
```

## How to verify

After creating a cluster, confirm all three cert-manager components are running:

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

Verify the ClusterIssuer is ready:

```sh
kubectl get clusterissuer selfsigned-issuer
```

Expected output:

```
NAME                READY   AGE
selfsigned-issuer   True    60s
```

## Configuration

cert-manager is controlled by the `addons.certManager` field in your cluster config:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  certManager: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

To create a cluster without cert-manager, set `certManager: false`:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  certManager: false
```

:::tip[Faster cluster creation]
Disabling cert-manager skips the webhook readiness wait, which can reduce cluster creation time by 30–60 seconds.
:::

## Technical notes

:::note[Server-side apply]
The cert-manager manifest is 986KB and exceeds Kubernetes' 256KB annotation limit for client-side apply. kinder uses `--server-side` apply automatically — no user action is needed, but this is why you may see `managedFields` with the `kubectl` manager in cert-manager resources.
:::
