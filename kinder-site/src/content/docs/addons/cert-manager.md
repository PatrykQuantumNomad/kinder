---
title: cert-manager
description: cert-manager addon for automatic TLS certificate management in kinder clusters.
---

cert-manager gives kinder clusters automatic TLS certificate management. A self-signed `ClusterIssuer` is created during cluster setup so you can issue `Certificate` resources immediately — no manual cert-manager installation or issuer configuration required.

kinder installs cert-manager **v1.20.2**.

:::caution[Breaking changes in v1.20]
Two upstream-default changes affect users upgrading from earlier kinder versions:

- **Container UID changed from 1000 to 65532.** cert-manager pods now run as the unprivileged UID `65532`. PVCs or mounted Secrets pre-populated with files owned by UID 1000 will be unreadable to the new pods. cert-manager itself does not use PVCs, but custom integrations that share volumes with cert-manager may break.
- **`Certificate.spec.privateKey.rotationPolicy: Always` is now GA-mandatory.** v1.18.0 changed the default from `Never` to `Always`; v1.20 makes it required (cannot disable). Long-running Certificates without an explicit `rotationPolicy: Never` will see a NEW private key on next renewal. Set `rotationPolicy: Never` explicitly if you want the old behavior — kinder does not patch this for you.
:::

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

## More certificate examples

### Wildcard certificate with custom duration

A wildcard certificate covers all subdomains under a single domain. This is useful when you have multiple services (e.g., `api.example.local`, `app.example.local`) and want a single cert to cover them all.

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-example-local
  namespace: default
spec:
  secretName: wildcard-example-local-tls
  duration: 2160h    # 90 days
  renewBefore: 360h  # renew 15 days before expiry
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  dnsNames:
    - "*.example.local"
    - "example.local"
```

:::note[Always use kind: ClusterIssuer]
kinder creates `selfsigned-issuer` as a `ClusterIssuer`, not a namespace-scoped `Issuer`. Always set `kind: ClusterIssuer` in the `issuerRef` block — using `kind: Issuer` will cause the certificate to stay `READY: False`.
:::

Apply it and verify it reaches `READY: True`:

```sh
kubectl apply -f wildcard-cert.yaml
kubectl get certificate wildcard-example-local
```

Expected output:

```
NAME                      READY   SECRET                        AGE
wildcard-example-local    True    wildcard-example-local-tls    15s
```

:::tip[Wildcard vs individual certs]
Use a wildcard cert when you have multiple subdomains under the same domain and want to manage a single secret. Use individual certs when services are in different namespaces (a secret can only be used in the namespace it is created in) or when you need per-service certificate lifetimes.
:::

## Troubleshooting

### Certificate stays READY: False

**Symptom:** `kubectl get certificate` shows `READY: False` and the status does not change.

Two common causes:

**(a) Webhook not ready yet**

cert-manager's webhook takes 30–60 seconds to become ready after cluster creation. If you apply a Certificate immediately after `kinder create cluster`, it may fail validation.

**Fix:** Wait 60 seconds and reapply, or check webhook readiness first:

```sh
kubectl get pods -n cert-manager
```

All three pods must be `Running` before applying certificates.

**(b) Wrong issuer kind**

The Certificate spec has `kind: Issuer` but `selfsigned-issuer` is a `ClusterIssuer`.

**Fix:** Change `kind: Issuer` to `kind: ClusterIssuer` in the `issuerRef` block:

```yaml
issuerRef:
  name: selfsigned-issuer
  kind: ClusterIssuer  # not Issuer
```

**Diagnostic commands:**

```sh
kubectl describe certificate <name>
kubectl get certificaterequest
```

`kubectl describe certificate` shows events that indicate whether the issue is with the issuer reference or a webhook timeout. `kubectl get certificaterequest` shows whether cert-manager created a request at all — if no request exists, the webhook likely rejected the Certificate resource.

## Technical notes

:::note[Server-side apply]
The cert-manager manifest is 986KB and exceeds Kubernetes' 256KB annotation limit for client-side apply. kinder uses `--server-side` apply automatically — no user action is needed, but this is why you may see `managedFields` with the `kubectl` manager in cert-manager resources.
:::
