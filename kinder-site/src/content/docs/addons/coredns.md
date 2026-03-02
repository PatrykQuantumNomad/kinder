---
title: CoreDNS Tuning
description: CoreDNS performance tuning applied by kinder to improve DNS resolution.
---

CoreDNS is already installed in every Kubernetes cluster. kinder does not install a new DNS server — it patches the existing CoreDNS ConfigMap to improve DNS resolution performance and reliability for local development workloads.

## What changes

kinder modifies three settings in the CoreDNS `Corefile`:

| Setting | Before | After | Effect |
|---|---|---|---|
| `autopath` | absent | `autopath @kubernetes` | Resolves short names to FQDNs using the cluster domain, reducing search-domain retry chains |
| `pods` | `pods insecure` | `pods verified` | Validates that the client IP matches the pod IP before resolving, preventing spoofed lookups |
| `cache` | `cache 30` | `cache 60` | Doubles the TTL for successful responses, reducing upstream DNS traffic |

### Before / after Corefile comparison

**Before (default kind CoreDNS Corefile):**

```corefile
.:53 {
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}
```

**After (kinder CoreDNS tuning applied):**

```corefile
.:53 {
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods verified
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
       autopath @kubernetes
    }
    prometheus :9153
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 60
    loop
    reload
    loadbalance
}
```

## How to verify

Inspect the live ConfigMap to confirm the tuning was applied:

```sh
kubectl get configmap coredns -n kube-system -o yaml
```

Look for `autopath @kubernetes` and `cache 60` in the `Corefile` data field. If both are present, the tuning is active.

## Configuration

CoreDNS tuning is controlled by the `addons.coreDNSTuning` field:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  coreDNSTuning: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  coreDNSTuning: false
```

The default kind CoreDNS configuration will be used without modification.
