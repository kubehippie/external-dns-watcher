# external-dns-watcher

![Version: 1.0.0](https://img.shields.io/badge/Version-1.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.0.0](https://img.shields.io/badge/AppVersion-1.0.0-informational?style=flat-square)

Watch any resource and generate External DNS records

**Homepage:** <https://github.com/kubehippie/external-dns-watcher>

## Installing the Chart

```console
helm install external-dns-watcher oci://ghcr.io/kubehippie/charts/external-dns-watcher
```

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| tboerger | <thomas@webhippie.de> | <https://github.com/tboerger> |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| annotations | object | `{}` | Define additional annotations |
| args | list | `["-leader-elect"]` | Arguments for the controller |
| certmanager.enable | bool | `false` | Enable Cert Manager integration |
| config | object | `{"watches":[{"group":"infrastructure.cluster.x-k8s.io","kind":"HetznerCluster","paths":[{"path":"$.status.controlPlaneLoadBalancer.ipv4","type":"A"},{"path":"$.status.controlPlaneLoadBalancer.ipv6","type":"AAAA"}],"recordTemplate":"{{ .Name }}-control-plane.example.com","version":"v1beta1"}]}` | Config mounted into the controller |
| crd.enable | bool | `true` | Install CRDs if we provide some |
| crd.keep | bool | `true` | Enable to add a helm.sh/resource-policy annotation |
| fullnameOverride | string | `""` | Override the fullname |
| image.imagePullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"ghcr.io/kubehippie/external-dns-watcher"` | Repository of the controller |
| image.tag | string | `""` | Tag of the controller |
| labels | object | `{}` | Define additional labels |
| livenessProbe | object | `{"httpGet":{"path":"/healthz","port":"healthz"},"initialDelaySeconds":15,"periodSeconds":20}` | Liveness probe for the controller |
| metrics.enable | bool | `true` | Enable the export of metrics |
| nameOverride | string | `""` | Override the name |
| networkPolicy.enable | bool | `false` | Enable builtin network policies |
| networkPolicy.from | list | `[{"namespaceSelector":{"matchLabels":{"metrics":"enabled"}}}]` | Match to allow the ingress traffic |
| podAnnotations | object | `{}` | Define additional annotations for the pod |
| podLabels | object | `{}` | Define additional labels for the pod |
| podSecurityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Security context for the controller |
| rbac.enable | bool | `true` | Enable role-based access control |
| rbac.extraRules | list | `[]` | Extra rules for the role of the controller |
| readinessProbe | object | `{"httpGet":{"path":"/readyz","port":"healthz"},"initialDelaySeconds":5,"periodSeconds":10}` | Readiness probe for the controller |
| replicas | int | `1` | Number of replicas for the controller |
| resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Resources available for the controller |
| securityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Security context for the deployment |
| serviceAccount.annotations | object | `{}` | Annotations for service account |
| serviceAccount.create | bool | `true` | Create a service account |
| serviceAccount.name | string | `nil` | Service account name |
| serviceMonitor.enable | bool | `false` | Enable a service monitor |
| updateStrategy | object | `{"type":"Recreate"}` | Update strategy for deployment |
