---
title: Headlamp Dashboard
description: Headlamp web dashboard addon for visual Kubernetes cluster management.
---

Headlamp is a lightweight, extensible Kubernetes web UI. kinder installs it automatically so you can browse resources, view logs, and inspect events without leaving the browser.

kinder installs Headlamp **v0.40.1**.

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
