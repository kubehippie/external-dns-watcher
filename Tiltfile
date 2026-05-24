allow_k8s_contexts('kind-external-dns-watcher')

docker_build(
  'ghcr.io/kubehippie/external-dns-watcher',
  '.',
  dockerfile='Dockerfile.tilt',
  entrypoint='/manager',
  live_update=[
    sync('.', '/workspace'),
    run(
      'go build -o /workspace/manager ./cmd/main.go',
      trigger=['cmd/', 'internal/', 'api/', 'go.mod', 'go.sum']
    ),
  ],
)

local_resource(
  'generate',
  'make generate',
  deps=[
    'api/**/**/*_types.go',
    'api/**/groupversion_info.go',
    'internal/',
    'cmd/',
    'hack/boilerplate.go.txt',
  ],
)

local_resource(
  'manifests',
  'make manifests',
  deps=[
    'api/**/**/*_types.go',
    'api/**/groupversion_info.go',
    'internal/',
  ],
  resource_deps=[
    'generate',
  ],
)

k8s_yaml(
  kustomize('config/default')
)

k8s_resource(
  'external-dns-watcher-controller-manager',
  extra_pod_selectors=[{'control-plane': 'controller-manager'}],
)
