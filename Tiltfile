load('ext://helm_resource', 'helm_resource')
allow_k8s_contexts('kind-external-dns-watcher')
update_settings(k8s_upsert_timeout_secs=5*60)

helm_resource(
  'external-dns',
  'external-dns',
  repo='https://kubernetes-sigs.github.io/external-dns/',
  namespace='external-dns',
  flags=[
    '--create-namespace',
    '--values=test/e2e/testdata/external-dns.yaml',
    '--timeout=5m',
    '--wait',
    '--hide-notes',
  ],
  labels=['dependencies'],
)

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

k8s_yaml(kustomize('config/default'))

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

k8s_resource(
  'external-dns-watcher-controller-manager',
  new_name='operator-manager',
  extra_pod_selectors=[{'control-plane': 'controller-manager'}],
  resource_deps=['external-dns'],
  labels=['operator'],
)
