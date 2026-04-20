# microdemo

A tiny 3-service Go microservices demo intended for OpenShift/Kubernetes deployments (Helm), optionally pre-wired for **Consul Connect** (transparent proxy) and **Consul intentions**.

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

## Prerequisites (OpenShift)

- An OpenShift cluster
- `oc`
- `helm`
- Container images accessible to the cluster (Quay/external registry)
- (Optional) Consul on Kubernetes/OpenShift (if you want Connect + intentions)

## Building container images

The `Dockerfile` builds a single binary selected by build arg `CMD` (defaults to `cmd/frontend`).

```bash
# frontend
docker build -t microdemo-frontend:local --build-arg CMD=cmd/frontend .

# api
docker build -t microdemo-api:local --build-arg CMD=cmd/api .

# worker
docker build -t microdemo-worker:local --build-arg CMD=cmd/worker .
```

The image listens on container port **8080**.

## Push images to Quay (example)

Tag and push each image to your Quay repository (replace `quay.io/YOUR_ORG/microdemo`).

```bash
# login
podman login quay.io

# tag
podman tag microdemo-frontend:local quay.io/YOUR_ORG/microdemo/frontend:latest
podman tag microdemo-api:local quay.io/YOUR_ORG/microdemo/api:latest
podman tag microdemo-worker:local quay.io/YOUR_ORG/microdemo/worker:latest

# push
podman push quay.io/YOUR_ORG/microdemo/frontend:latest
podman push quay.io/YOUR_ORG/microdemo/api:latest
podman push quay.io/YOUR_ORG/microdemo/worker:latest
```

> If your images are private, create an image pull secret in the OpenShift project and link it to the default service account, e.g.:
>
> ```bash
> oc -n demo create secret docker-registry quay-pull \
>   --docker-server=quay.io \
>   --docker-username=YOUR_USER \
>   --docker-password=YOUR_TOKEN_OR_PASSWORD \
>   --docker-email=you@example.com
>
> oc -n demo secrets link default quay-pull --for=pull
> ```

## Deploy to OpenShift with Helm

The Helm chart lives in `microdemo-chart/`.

### 1) Create a project (namespace)

The chart’s default values use:

- `namespace: demo`

Create it (recommended on OpenShift):

```bash
oc new-project demo
```

> Tip: If you use a different project name, set `--namespace <name>` when installing.

### 2) Configure images

Set these values (from `microdemo-chart/values.yaml`):

- `images.frontend`
- `images.api`
- `images.worker`

Example:

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set images.frontend=quay.io/YOUR_ORG/microdemo/frontend:latest \
  --set images.api=quay.io/YOUR_ORG/microdemo/api:latest \
  --set images.worker=quay.io/YOUR_ORG/microdemo/worker:latest
```

### 3) Enable an OpenShift Route for the frontend

The chart includes `templates/route.yaml` to create a Route to `frontend`.

Enable it:

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set route.enabled=true
```

If you want to pin the hostname:

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set route.enabled=true \
  --set route.host=microdemo.apps.YOUR_CLUSTER_DOMAIN
```

### 4) Verify

```bash
oc -n demo get pods
oc -n demo get svc
oc -n demo get route
```

Get the URL:

```bash
oc -n demo get route frontend -o jsonpath='{.spec.host}{"\n"}'
```

Then call it:

```bash
HOST="$(oc -n demo get route frontend -o jsonpath='{.spec.host}')"
curl -s "http://$HOST/" | jq .
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

### Namespace labeling for Consul injection

If you are using Consul Connect sidecar injection, ensure the project is labeled:

```bash
oc label namespace demo consul.hashicorp.com/connect-inject=true --overwrite
```

If you are not using Consul, you can disable Consul resources:

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set consul.enabled=false
```

## Configuration (env vars)

These are set in the Helm Deployments:

- `frontend`
  - `API_URL` (default in code: `http://api:8080/`)
- `api`
  - `WORKER_URL` (default in code: `http://worker:8080/`)

## Troubleshooting (OpenShift)

### Check health endpoints from inside the cluster

```bash
oc -n demo run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl -sS http://frontend:8080/healthz
```

Similarly:

```bash
oc -n demo run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl -sS http://api:8080/healthz
```

```bash
oc -n demo run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl -sS http://worker:8080/healthz
```

### Logs

```bash
oc -n demo logs deploy/frontend
oc -n demo logs deploy/api
oc -n demo logs deploy/worker
```

### Debug header propagation / request id

The `api` service propagates `X-Request-Id` to the worker (and generates one if missing), and returns it in the response header.

```bash
HOST="$(oc -n demo get route frontend -o jsonpath='{.spec.host}')"
curl -i -H "X-Request-Id: test-123" "http://$HOST/"
```

## License

Add a license if you intend to publish this broadly.
