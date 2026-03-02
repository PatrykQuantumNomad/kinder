<p align="center"><img alt="kinder" src="./kinder-logo/logo.png" width="300px" /></p>

# kinder — kind, but with everything you actually need.

[![Documentation](https://img.shields.io/badge/docs-kinder.patrykgolabek.dev-00B8D4)](https://kinder.patrykgolabek.dev)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

kinder is a batteries-included tool for running local Kubernetes clusters using Docker container "nodes". Built on top of [kind], it pre-installs production-ready addons so you can go from zero to a fully functional cluster in one command.

## What you get

| Addon | What it does |
|-------|-------------|
| [MetalLB](https://kinder.patrykgolabek.dev/addons/metallb/) | LoadBalancer services with real external IPs from your Docker/Podman subnet |
| [Envoy Gateway](https://kinder.patrykgolabek.dev/addons/envoy-gateway/) | Gateway API routing with the `eg` GatewayClass pre-configured |
| [Metrics Server](https://kinder.patrykgolabek.dev/addons/metrics-server/) | Enables `kubectl top` and Horizontal Pod Autoscaler support |
| [CoreDNS Tuning](https://kinder.patrykgolabek.dev/addons/coredns/) | Autopath, verified pod records, and doubled cache TTL |
| [Headlamp](https://kinder.patrykgolabek.dev/addons/headlamp/) | Web-based cluster dashboard accessible via port-forward |

## Quick start

### Prerequisites

- [Go] 1.25.7+ &nbsp;|&nbsp; [Docker], [Podman], or [nerdctl] &nbsp;|&nbsp; [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Install

```sh
git clone https://github.com/PatrykQuantumNomad/kinder.git
cd kinder
make install
```

### Create a cluster

```sh
kinder create cluster
```

That's it. All addons are installed automatically.

### Verify

```sh
kubectl get nodes                          # node is Ready
kubectl get pods -n metallb-system         # MetalLB running
kubectl get pods -n envoy-gateway-system   # Envoy Gateway running
kubectl top nodes                          # Metrics Server working
```

### Open the dashboard

```sh
kubectl port-forward -n kube-system service/headlamp 8080:80
```

Open [http://localhost:8080](http://localhost:8080) and paste the token printed during cluster creation.

### Delete the cluster

```sh
kinder delete cluster
```

## Configuration

kinder uses the same `kind.x-k8s.io/v1alpha4` config format as kind. Existing kind config files work without modification.

To disable specific addons:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  dashboard: false
  envoyGateway: false
```

See the full [Configuration Reference](https://kinder.patrykgolabek.dev/configuration/).

## Why kinder over plain kind?

- **One command** — no post-install scripts to wire up MetalLB, ingress, metrics, or a dashboard
- **LoadBalancer support** — `type: LoadBalancer` works out of the box
- **Gateway API** — Envoy Gateway ready without manual CRD installation
- **Observability** — `kubectl top` and a web dashboard from the start
- **100% compatible** — any kind config or workflow still works

## Documentation

Full docs at **[kinder.patrykgolabek.dev](https://kinder.patrykgolabek.dev)**

## Contributing

Issues and pull requests are welcome at [github.com/PatrykQuantumNomad/kinder](https://github.com/PatrykQuantumNomad/kinder).

## Acknowledgements

kinder is a fork of [kind] by the Kubernetes SIG-Testing community. All credit for the core cluster lifecycle, node images, and kubeadm bootstrapping goes to the original [kind maintainers](https://kind.sigs.k8s.io/).

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.

---

<p align="center">
  Built by <a href="https://patrykgolabek.dev">Patryk Golabek</a>
</p>

<!--links-->
[kind]: https://kind.sigs.k8s.io/
[Go]: https://go.dev/
[Docker]: https://www.docker.com/
[Podman]: https://podman.io/
[nerdctl]: https://github.com/containerd/nerdctl
