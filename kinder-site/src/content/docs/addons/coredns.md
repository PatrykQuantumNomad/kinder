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

## Practical examples

### Verify DNS resolution from a pod

To confirm CoreDNS is resolving cluster-internal names, run an `nslookup` from inside a temporary pod:

```sh
kubectl run dns-test --image=busybox:1.36 --rm -it --restart=Never -- nslookup kubernetes.default
```

Expected output:

```
Server:    10.96.0.10
Address 1: 10.96.0.10 kube-dns.kube-system.svc.cluster.local

Name:      kubernetes.default
Address 1: 10.96.0.1 kubernetes.default.svc.cluster.local
```

The `Server` IP (`10.96.0.10`) is the CoreDNS cluster IP. Seeing it means your pod is hitting CoreDNS, not host DNS.

:::caution
DNS lookups must be run from **inside a pod**, not from the host. Running `nslookup kubernetes.default` directly on your Mac or Linux host queries host DNS, which does not know about cluster-internal names and will return an error.
:::

### Test short-name resolution

The `autopath @kubernetes` setting kinder applies eliminates the NXDOMAIN retry chain for short names. To verify it works:

```sh
kubectl run dns-test2 --image=busybox:1.36 --rm -it --restart=Never -- nslookup kubernetes
```

Without `autopath`, Kubernetes resolves short names by appending each search domain in turn (`kubernetes.default.svc.cluster.local`, `kubernetes.svc.cluster.local`, etc.) and retrying on NXDOMAIN. With `autopath @kubernetes`, CoreDNS resolves the FQDN directly and returns the answer without the retry chain, reducing DNS lookup latency.

## Troubleshooting

### DNS resolution fails inside pods

**Symptom:** Running `nslookup` from inside a pod returns `server can't find <name>` or the command times out.

**Cause:** CoreDNS pods may not be running, or the Corefile may not have been patched correctly by kinder.

**Fix:**

Check that CoreDNS pods are running:

```sh
kubectl get pods -n kube-system -l k8s-app=kube-dns
```

Both pods should show `Running`. If either is in `CrashLoopBackOff`, describe the pod for errors.

Confirm the Corefile contains the kinder tuning:

```sh
kubectl get configmap coredns -n kube-system -o yaml
```

Look for `autopath @kubernetes` inside the `Corefile` data field. If it is absent, the kinder CoreDNS tuning was not applied — this can happen if `coreDNSTuning` was set to `false` in the cluster config.
