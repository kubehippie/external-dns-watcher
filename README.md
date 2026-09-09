# external-dns-watcher

[![GitHub Repo](https://img.shields.io/badge/github-repo-yellowgreen)](https://github.com/kubehippie/external-dns-watcher) [![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/external-dns-watcher)](https://artifacthub.io/packages/helm/external-dns-watcher/external-dns-watcher)

This small controller can watch a configurable set of resources within a
Kubernetes cluster and generate `DNSEndpoint` resources which are part of
[External DNS][external-dns]. Our primary use case that gets covered are
resources like `HetznerCluster` created by [Cluster API][cluster-api] where we
want to generate DNS records automatically as we don't like to use plain IP
addresses.

## Instructions

Generally you should install this project via [Helm][helm], the other options
are not covered by this document as the chart deployment is the preferred way:

```sh
cat << EOF | helm install external-dns-watcher oci://ghcr.io/kubehippie/charts/external-dns-watcher --values -
fullnameOverride: external-dns-watcher

rbac:
  extraRules:
    - apiGroups:
        - infrastructure.cluster.x-k8s.io
    resources:
        - hetznerclusters
    verbs:
        - get
        - list
        - watch

config:
  watches:
    - group: infrastructure.cluster.x-k8s.io
      version: v1beta1
      kind: HetznerCluster
      recordTemplate: "{{ .Name }}-control-plane.example.com"
      paths:
        - path: "$.status.controlPlaneLoadBalancer.ipv4"
          type: A
        - path: "$.status.controlPlaneLoadBalancer.ipv6"
          type: AAAA
EOF
```

If you want to watch different kinds of resources you got to define the watch
rules and also the required extra RBAC rules, otherwise the operator is not able
to read the sources. The watch definitions can always use a JSONPath to match
the value for the DNS records.

## Development

We are using [Mise][mise] to install all required tools with fixed versions to
keep everything as far as possible compatible. If you don't want to use
[Mise][mise] it is up to you to install the required tools like Go. Beside that
we are using `make` to define all commands to build this project.

```console
git clone https://github.com/kubehippie/external-dns-watcher.git
cd external-dns-watcher

mise trust
mise install

make build
./bin/manager -h
```

To easily work on the operator we suggest to use [Tilt][tilt] for the local
development, this work pretty good in combination with Kind to get features like
hot reloading:

```console
kind create cluster \
    --name external-dns-watcher

tilt up

kind delete cluster \
    --name external-dns-watcher
```

## Security

If you find a security issue please contact
[thomas@webhippie.de](mailto:thomas@webhippie.de) first.

## Contributing

Fork -> Patch -> Push -> Pull Request

## Authors

-   [Thomas Boerger](https://github.com/tboerger)

## License

Apache-2.0

## Copyright

```console
Copyright (c) 2025 Thomas Boerger <thomas@webhippie.de>
```

[external-dns]: https://kubernetes-sigs.github.io/external-dns/
[cluster-api]: https://cluster-api.sigs.k8s.io/
[helm]: https://helm.sh/
[mise]: https://mise.jdx.dev/getting-started.html
[tilt]: https://tilt.dev/
