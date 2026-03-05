# hashrouter

Minimal HTTP service for deterministic routing of traffic to specific pods (for example, distributed cache use cases).

## Endpoints

- `GET /` returns plain text: `hashrouter dev`
- `GET /{namespace}/{service}/{path...}` resolves endpoints, selects one via rendezvous hashing, and responds with `307` redirect to `http://{ip}:{port}/{path...}`
- `GET /healthz` returns JSON health payload: `{"health":"ok"}`

## Run

```bash
go run ./cmd/hashrouter
```

Set `PORT` to override the default `8080`.
Set `DEBUG=true` to enable debug mode in configuration loading.

## Release

- `make show_latest_tag` prints the latest git tag.
- `make release` prompts for a semver tag (`vX.Y.Z`), updates `deploy/deployment.yaml` image tag, commits it, pushes, tags, and runs GoReleaser.
- GoReleaser config lives in `.goreleaser.yaml` and uses `Dockerfile.goreleaser`.

## Kubernetes RBAC

Apply RBAC for cluster-wide service and endpoint slice informers:

```bash
kubectl apply -f deploy/rbac.yaml
```

Deploy the application and internal ClusterIP service:

```bash
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```
