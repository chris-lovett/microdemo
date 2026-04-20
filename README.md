# microdemo

A tiny 3-service Go microservices demo intended for Kubernetes deployments (Helm), optionally pre-wired for **Consul Connect** (transparent proxy) and **Consul intentions**.

Services and call chain:

- **frontend** (HTTP :8080) calls **api**
- **api** (HTTP :8080) calls **worker**
- **worker** (HTTP :8080) returns a small JSON payload

Each service exposes:

- `GET /` – JSON response (frontend combines downstream responses)
- `GET /healthz` – health endpoint used by Kubernetes probes
- `GET /headers` – returns request headers as JSON (useful for debugging propagation)

## Repository layout

- `cmd/frontend/main.go` – frontend service
- `cmd/api/main.go` – api service (propagates `X-Request-Id` to worker)
- `cmd/worker/main.go` – worker service
- `Dockerfile` – builds one of the services via `--build-arg CMD=cmd/<service>`
- `microdemo-chart/` – Helm chart for deploying all three services
- `deploy/k8s.yaml` – (currently empty placeholder)

## Prerequisites

Local build/run:

- Go **1.22+**
- Docker (optional)

Kubernetes deploy:

- Kubernetes cluster
- `kubectl`
- `helm`
- (Optional) Consul on Kubernetes (if you want Connect + intentions)
- (Optional) OpenShift (if you want to use the included `Route` template)

## Local development (no Kubernetes)

You can run the services directly with Go in three terminals.

### Terminal 1: worker

```bash
go run ./cmd/worker
```

### Terminal 2: api

```bash
WORKER_URL="http://localhost:8082/" go run ./cmd/api
```

…but note: by default the worker listens on `:8080` too. If you want to run all three locally at once without port conflicts, either:
- run each on a different port (requires a small code change), or
- run in containers, or
- run on Kubernetes (recommended for this demo)

Because the current code hard-codes `addr := ":8080"` in all three services, the simplest “multi-service” local run is via Kubernetes or by running each in a container with different published ports.

## Building container images

The `Dockerfile` builds a single binary selected by build arg `CMD` (defaults to `cmd/frontend`).

Examples:

```bash
# frontend
docker build -t microdemo-frontend:local --build-arg CMD=cmd/frontend .

# api
docker build -t microdemo-api:local --build-arg CMD=cmd/api .

# worker
docker build -t microdemo-worker:local --build-arg CMD=cmd/worker .
```

The image listens on container port **8080**.

## Helm deployment (Kubernetes)

The Helm chart lives in `microdemo-chart/`.

### 1) Choose / create a namespace

By default the chart uses the namespace in values:

- `namespace: demo`

The chart can optionally create the namespace (`createNamespace: true`), otherwise create it yourself:

```bash
kubectl create namespace demo
```

### 2) (Optional) Enable Consul sidecar injection for the namespace

The chart includes a `values.yaml` switch:

- `labelNamespaceForConsulInject: true`

This is intended to ensure the namespace gets labeled for injection:

- `consul.hashicorp.com/connect-inject=true`

Important: in the current chart templates, I only see a Namespace template (created when `createNamespace: true`) and no Hook Job template that actually labels an existing namespace. If your namespace already exists, make sure it is labeled:

```bash
kubectl label namespace demo consul.hashicorp.com/connect-inject=true --overwrite
```

If you are not using Consul Connect injection, you can disable Consul resources:

```bash
--set consul.enabled=false
```

### 3) Configure images

Default images are configured in `microdemo-chart/values.yaml`:

- `images.frontend`
- `images.api`
- `images.worker`

Example install using your own tags:

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set images.frontend=quay.io/chris_lovett/microdemo/frontend:latest \
  --set images.api=quay.io/chris_lovett/microdemo/api:latest \
  --set images.worker=quay.io/chris_lovett/microdemo/worker:latest
```

### 4) Install / upgrade

```bash
helm upgrade --install microdemo ./microdemo-chart --namespace demo
```

### 5) Verify pods and services

```bash
kubectl -n demo get pods
kubectl -n demo get svc
```

You should see Deployments and Services:

- `frontend`, `api`, `worker` (each Service is port 8080)

### 6) Access the frontend

#### Option A: Port-forward (works on any Kubernetes)

```bash
kubectl -n demo port-forward svc/frontend 8080:8080
```

Then:

```bash
curl -s http://localhost:8080/ | jq .
curl -i http://localhost:8080/ | sed -n '1,20p'   # observe X-Request-Id header returned by api
```

#### Option B: OpenShift Route (only if you’re on OpenShift)

The chart includes `microdemo-chart/templates/route.yaml` which creates an OpenShift `Route` for `frontend` when enabled:

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set route.enabled=true \
  --set route.host=your.host.example.com
```

If `route.host` is left empty, OpenShift will generate a host.

Check:

```bash
oc -n demo get route frontend
```

## Consul (optional)

If `consul.enabled=true`, the chart renders:

- `ServiceDefaults` for `frontend`, `api`, `worker` with `protocol: http`
- `ServiceIntentions` that can enforce **default deny** and then explicitly allow:
  - `frontend -> api`
  - `api -> worker`

Key values:

- `consul.enabled` (default: true)
- `consul.intentions.enabled` (default: true)
- `consul.intentions.defaultDeny` (default: true)

If you enable default deny, these intentions are required for the app to function.

## Configuration (env vars)

These are set in the Helm Deployments:

- `frontend`
  - `API_URL` (default in code: `http://api:8080/`)
- `api`
  - `WORKER_URL` (default in code: `http://worker:8080/`)

## Troubleshooting

### Check health endpoints

```bash
kubectl -n demo run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl -sS http://frontend:8080/healthz
```

Similarly:

```bash
curl -sS http://api:8080/healthz
curl -sS http://worker:8080/healthz
```

### Look at logs

```bash
kubectl -n demo logs deploy/frontend
kubectl -n demo logs deploy/api
kubectl -n demo logs deploy/worker
```

### Debug header propagation / request id

The `api` service propagates `X-Request-Id` to the worker (and generates one if missing), and returns it in the response header.

Try:

```bash
curl -i -H "X-Request-Id: test-123" http://localhost:8080/
```

Then inspect the JSON returned from `worker` to confirm it received the request id.

## License

Add a license if you intend to publish this broadly.
