---
title: Headlamp Dashboard
description: Headlamp web dashboard addon for visual Kubernetes cluster management.
---

Headlamp is a lightweight, extensible Kubernetes web UI. kinder installs it automatically so you can browse resources, view logs, and inspect events without leaving the browser.

kinder installs Headlamp **v0.42.0**.

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| `deployment/headlamp` | `kube-system` | The Headlamp web server |
| `ServiceAccount` headlamp | `kube-system` | Identity for the dashboard |
| `ClusterRoleBinding` headlamp | cluster-scoped | Read access to all resources |
| `Secret` kinder-dashboard-token | `kube-system` | Static bearer token for login |

## Accessing the dashboard

Headlamp is not exposed externally. Use `kubectl port-forward` to access it locally:

```sh
kubectl port-forward -n kube-system svc/headlamp 8080:80
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## Retrieving the login token

The dashboard requires a bearer token to authenticate. kinder creates a static token secret called `kinder-dashboard-token` during cluster setup.

:::tip[Get the token]
Run the following command to retrieve the token:

```sh
kubectl get secret kinder-dashboard-token -n kube-system \
  -o jsonpath='{.data.token}' | base64 --decode
```

Copy the output and paste it into the Headlamp token field when prompted.
:::

## How to verify

Check that the Headlamp pod is running:

```sh
kubectl get pods -n kube-system -l app.kubernetes.io/name=headlamp
```

Expected output:

```
NAME                        READY   STATUS    RESTARTS   AGE
headlamp-...                1/1     Running   0          60s
```

Confirm the token secret exists:

```sh
kubectl get secret kinder-dashboard-token -n kube-system
```

## Configuration

Headlamp is controlled by the `addons.dashboard` field:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  dashboard: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  dashboard: false
```

## What you can do

Once logged in, Headlamp provides a full cluster overview. Here are the main areas you will use:

- **Workloads** — View Deployments, Pods, ReplicaSets, DaemonSets, Jobs. Click any pod to see its status, containers, and resource usage.
- **Logs** — Click a pod, then select a container to stream its logs in real time. Use the search bar to filter log lines.
- **Events** — The cluster Events view shows recent scheduling decisions, image pulls, and errors across all namespaces. Useful for debugging pod startup failures.
- **Config** — Browse ConfigMaps and Secrets. kinder's `kinder-dashboard-token` secret appears here under `kube-system`.
- **Nodes** — View node status, capacity, and allocated resources. In a single-node kinder cluster, this shows the control-plane node.

## Troubleshooting

### Invalid token error

**Symptom:** Headlamp shows "Invalid token" or "Unauthorized" after pasting the token.

**Cause:** The raw base64-encoded value was pasted instead of the decoded token. The `kubectl get secret` command returns base64-encoded data; the `| base64 --decode` step is required.

**Fix:** Re-run the token retrieval command ensuring the `| base64 --decode` pipe is included:

```sh
kubectl get secret kinder-dashboard-token -n kube-system \
  -o jsonpath='{.data.token}' | base64 --decode
```

Copy the decoded output (a long JWT string) and paste it into the token field.
