# external-dns-watcher

[![GitHub Repo](https://img.shields.io/badge/github-repo-yellowgreen)](https://github.com/kubehippie/external-dns-watcher) [![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kubehippie)](https://artifacthub.io/packages/helm/kubehippie/external-dns-watcher)

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
cat << EOF > values.yaml
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

helm install external-dns-watcher oci://ghcr.io/kubehippie/charts/external-dns-watcher --values values.yaml
```

If you want to watch different kinds of resources you got to define the watch
rules and also the required extra RBAC rules, otherwise the operator is not able
to read the sources. The watch definitions can always use a JSONPath to match
the value for the DNS records.

## Development

If you are not familiar with [Nix][nix] it is up to you to have a working
environment for Go (>= 1.24.0) as the setup won't be covered within this guide.
Please follow the official install instructions for [Go][golang] and. Beside
that we are using `make` to define all commands to build this project.

```console
git clone https://github.com/kubehippie/external-dns-watcher.git
cd external-dns-watcher

make build
./bin/manager -h
```

If you got [Nix][nix] and [Direnv][direnv] configured you can simply execute
the following commands to get all dependencies including `make` and the required
runtimes installed:

```console
cat << EOF > .envrc
use flake . --impure
EOF

direnv allow
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
[nix]: https://nixos.org/
[golang]: http://golang.org/doc/install.html
[direnv]: https://direnv.net/
