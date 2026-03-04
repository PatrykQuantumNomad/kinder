---
title: HPA Auto-Scaling
description: Set up Horizontal Pod Autoscaler with Metrics Server and watch pods scale under load.
---

In this tutorial you will deploy a CPU-intensive workload, configure a Horizontal Pod Autoscaler (HPA), generate load against it, and watch Kubernetes automatically scale pods up in response. When the load stops, you will observe pods scale back down. Metrics Server — pre-installed by kinder — powers the CPU metrics that make this possible.

## Prerequisites

- [kinder installed](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH

## Step 1: Create the cluster

```sh
kinder create cluster
```

:::tip
Metrics Server is enabled by default in every kinder cluster. You do not need to install it manually or pass any flags.
:::

## Step 2: Deploy a CPU-intensive application

The `registry.k8s.io/hpa-example` image runs a PHP script that performs CPU-intensive calculations on every request — it is the standard Kubernetes HPA demo workload.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: php-apache
spec:
  replicas: 1
  selector:
    matchLabels:
      app: php-apache
  template:
    metadata:
      labels:
        app: php-apache
    spec:
      containers:
        - name: php-apache
          image: registry.k8s.io/hpa-example
          ports:
            - containerPort: 80
          resources:
            requests:
              cpu: "200m"
            limits:
              cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: php-apache
spec:
  selector:
    app: php-apache
  ports:
    - port: 80
      targetPort: 80
```

Save this as `php-apache.yaml` and apply it:

```sh
kubectl apply -f php-apache.yaml
```

Wait for the pod to be ready:

```sh
kubectl get pods
```

Expected output:

```
NAME                          READY   STATUS    RESTARTS   AGE
php-apache-7d8b9c6f5-xk4pq   1/1     Running   0          30s
```

:::caution
The deployment **must** have `resources.requests.cpu` set. Without it, the HPA cannot calculate CPU utilization and will show `<unknown>/50%` in the TARGETS column. Resource requests are the denominator in the HPA's utilization calculation: `current CPU / requested CPU`.
:::

## Step 3: Create a Horizontal Pod Autoscaler

Create an HPA that targets the `php-apache` deployment, scaling between 1 and 5 replicas when CPU utilization crosses 50%.

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: php-apache-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: php-apache
  minReplicas: 1
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
```

Save this as `php-apache-hpa.yaml` and apply it:

```sh
kubectl apply -f php-apache-hpa.yaml
```

Verify the HPA was created:

```sh
kubectl get hpa php-apache-hpa
```

Expected output (TARGETS may show `<unknown>/50%` initially):

```
NAME             REFERENCE               TARGETS         MINPODS   MAXPODS   REPLICAS   AGE
php-apache-hpa   Deployment/php-apache   <unknown>/50%   1         5         1          10s
```

## Step 4: Verify the HPA is reading metrics

Wait 60 seconds for Metrics Server to collect its first scrape cycle, then check the HPA again:

```sh
kubectl get hpa php-apache-hpa
```

Expected output:

```
NAME             REFERENCE               TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
php-apache-hpa   Deployment/php-apache   8%/50%    1         5         1          90s
```

:::note
If TARGETS still shows `<unknown>/50%` after 60 seconds, wait another 30 seconds and check again. Metrics Server scrapes on a 15-second interval and needs a few cycles before values stabilize. If it continues to show unknown after 2 minutes, verify that `resources.requests.cpu` is set on the deployment container.
:::

## Step 5: Generate load

In a separate terminal, start a load-generating pod inside the cluster. This pod runs a continuous loop that sends requests to the `php-apache` service, driving up its CPU usage:

```sh
kubectl run load-generator \
  --image=busybox:1.28 \
  --restart=Never \
  -- /bin/sh -c "while true; do wget -qO- http://php-apache; done"
```

The `load-generator` pod runs inside the cluster and can reach `php-apache` directly by service name. The continuous `wget` loop generates sustained CPU load on the PHP containers.

Verify the load generator is running:

```sh
kubectl get pod load-generator
```

Expected output:

```
NAME             READY   STATUS    RESTARTS   AGE
load-generator   1/1     Running   0          15s
```

## Step 6: Watch pods scale up

In a separate terminal, watch the HPA as it responds to the increasing CPU utilization:

```sh
kubectl get hpa php-apache-hpa --watch
```

Expected output (over several minutes as load builds):

```
NAME             REFERENCE               TARGETS    MINPODS   MAXPODS   REPLICAS   AGE
php-apache-hpa   Deployment/php-apache   8%/50%     1         5         1          2m
php-apache-hpa   Deployment/php-apache   62%/50%    1         5         1          2m30s
php-apache-hpa   Deployment/php-apache   62%/50%    1         5         2          2m45s
php-apache-hpa   Deployment/php-apache   98%/50%    1         5         2          3m
php-apache-hpa   Deployment/php-apache   98%/50%    1         5         4          3m15s
php-apache-hpa   Deployment/php-apache   51%/50%    1         5         4          3m45s
php-apache-hpa   Deployment/php-apache   48%/50%    1         5         4          4m
```

TARGETS rises above 50%, the HPA increases REPLICAS, and CPU per pod drops as the load is spread across more instances. The exact numbers will differ based on your machine's speed.

You can also watch the pods being created in real time:

```sh
kubectl get pods --watch
```

Expected output (scaling up):

```
NAME                          READY   STATUS    RESTARTS   AGE
php-apache-7d8b9c6f5-xk4pq   1/1     Running   0          4m
php-apache-7d8b9c6f5-bv9mz   1/1     Running   0          45s
php-apache-7d8b9c6f5-cr7jl   1/1     Running   0          45s
php-apache-7d8b9c6f5-dt2wn   1/1     Running   0          45s
```

## Step 7: Stop load and observe scale-down

Delete the load-generator pod to stop the traffic:

```sh
kubectl delete pod load-generator
```

Expected output:

```
pod "load-generator" deleted
```

Wait a few minutes, then check the HPA:

```sh
kubectl get hpa php-apache-hpa
```

Expected output:

```
NAME             REFERENCE               TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
php-apache-hpa   Deployment/php-apache   0%/50%    1         5         1          12m
```

:::note[Scale-down takes ~5 minutes]
Kubernetes deliberately delays scale-down to avoid thrashing — rapidly adding and removing pods in response to short bursts. The default stabilization window is 5 minutes. After you stop the load, REPLICAS will return to 1 within approximately 5 minutes. This is expected behavior, not a bug.
:::

## Clean up

```sh
kinder delete cluster
```

This removes the cluster, all pods, and the HPA. No other cleanup is required.
