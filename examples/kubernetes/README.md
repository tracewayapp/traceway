# Traceway for Kubernetes

Ready-to-apply OpenTelemetry Collector manifests that report a whole cluster to a
Traceway project: host metrics per node, pod resource usage, pod logs, cluster
state, and Kubernetes events.

Full walkthrough: [docs.tracewayapp.com/learn/kubernetes](https://docs.tracewayapp.com/learn/kubernetes)

## Files

| File | What it deploys |
|---|---|
| `traceway-kubernetes.yaml` | `traceway-node-agent` (DaemonSet) and `traceway-cluster-agent` (Deployment), plus namespace, ServiceAccount and RBAC |
| `traceway-gateway.yaml` | Optional in-cluster OTLP endpoint your applications export to, so the project token lives in one Secret |
| `helm/node-agent-values.yaml` | The same node agent as values for the upstream `open-telemetry/opentelemetry-collector` chart |
| `helm/cluster-agent-values.yaml` | The same cluster agent as chart values |

## Install

```bash
kubectl create namespace traceway

kubectl create secret generic traceway-token \
  --namespace traceway \
  --from-literal=token='<your-project-token>'

# Set TRACEWAY_CLUSTER_NAME and TRACEWAY_ENDPOINT in the traceway-settings
# ConfigMap at the top of the file first.
kubectl apply -f traceway-kubernetes.yaml
```

The project token is the one from your Traceway project's **Settings** page.
`TRACEWAY_ENDPOINT` is your instance's OTLP base URL, ending in `/api/otel`.

## What you get

- One instance row per node in the organization overview, grouped under
  `TRACEWAY_CLUSTER_NAME`, with CPU, memory, disk and network.
- `k8s.pod.*` metrics for per-pod dashboards.
- Container stdout/stderr on the Logs page.
- Cluster state metrics and Kubernetes events.

## Notes

- `TRACEWAY_CLUSTER_NAME` becomes `k8s.cluster.name`. Use a distinct value per
  cluster; that string is what the overview groups by.
- The node agent sets `service.name` to the node name, which becomes the
  `server_name` tag. That is what gives one instance row per node.
- The three `system.*.utilization` metrics are off by default in the hostmetrics
  receiver and are enabled explicitly in these configs. Without them the
  organization overview has no CPU, memory or disk to show.
- Keep `traceway-cluster-agent` at one replica.
- The collector image is pinned to a released version. Bump it deliberately.
