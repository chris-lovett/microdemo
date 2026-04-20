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
docker build -t microdemo-frontend:0.1.0 --build-arg CMD=cmd/frontend .

# api
docker build -t microdemo-api:0.1.0 --build-arg CMD=cmd/api .

# worker
docker build -t microdemo-worker:0.1.0 --build-arg CMD=cmd/worker .
```

The image listens on container port **8080**.

### Multi-arch images (required for amd64 OpenShift nodes)

If you build on Apple Silicon (arm64) and deploy to an amd64 OpenShift cluster you will see:

```
no image found in image index for architecture "amd64"
```

Use `docker buildx` to build and push a multi-arch manifest in one step (replace `quay.io/YOUR_ORG/microdemo` and the version tag):

```bash
docker buildx create --use --name multiarch || true

# frontend
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg CMD=cmd/frontend \
  -t quay.io/YOUR_ORG/microdemo/frontend:0.1.0 \
  --push .

# api
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg CMD=cmd/api \
  -t quay.io/YOUR_ORG/microdemo/api:0.1.0 \
  --push .

# worker
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg CMD=cmd/worker \
  -t quay.io/YOUR_ORG/microdemo/worker:0.1.0 \
  --push .
```

> If you only need amd64, replace `linux/amd64,linux/arm64` with `linux/amd64`.

## Push images to Quay (example)

Tag and push each image to your Quay repository (replace `quay.io/YOUR_ORG/microdemo`).

> **Image tag guidance:** Always use the exact tag that was pushed (e.g. `0.1.0`).
> Using `:latest` will cause `manifest unknown` / `not found` errors if only a versioned tag was pushed.
> Either use an explicit version tag (recommended) **or** also push a `:latest` tag alongside.

```bash
# login
podman login quay.io

# tag (using explicit version tag)
podman tag microdemo-frontend:0.1.0 quay.io/YOUR_ORG/microdemo/frontend:0.1.0
podman tag microdemo-api:0.1.0      quay.io/YOUR_ORG/microdemo/api:0.1.0
podman tag microdemo-worker:0.1.0   quay.io/YOUR_ORG/microdemo/worker:0.1.0

# push
podman push quay.io/YOUR_ORG/microdemo/frontend:0.1.0
podman push quay.io/YOUR_ORG/microdemo/api:0.1.0
podman push quay.io/YOUR_ORG/microdemo/worker:0.1.0

# optional: also tag and push :latest so that both tags resolve
podman tag quay.io/YOUR_ORG/microdemo/frontend:0.1.0 quay.io/YOUR_ORG/microdemo/frontend:latest
podman push quay.io/YOUR_ORG/microdemo/frontend:latest
# (repeat for api and worker)
```

### Quay pull secret (private images)

If your Quay repositories are private, create an image pull secret and link it to the service account used by the deployments.

**Recommended:** use a [Quay robot account](https://docs.quay.io/glossary/robot-accounts.html) with read access to the image repos instead of your personal credentials.

```bash
# Create the pull secret (robot account token recommended)
oc -n demo create secret docker-registry quay-pull \
  --docker-server=quay.io \
  --docker-username='YOUR_ORG+robot_account_name' \
  --docker-password=ROBOT_TOKEN

# Confirm which ServiceAccount the pods use (typically "default"; check each deployment)
oc -n demo get deploy frontend -o jsonpath='{.spec.template.spec.serviceAccountName}{"\n"}'
oc -n demo get deploy api     -o jsonpath='{.spec.template.spec.serviceAccountName}{"\n"}'
oc -n demo get deploy worker  -o jsonpath='{.spec.template.spec.serviceAccountName}{"\n"}'

# Link the pull secret to that service account
oc -n demo secrets link default quay-pull --for=pull

# Verify the secret is listed
oc -n demo get sa default -o jsonpath='{.imagePullSecrets}{"\n"}'
```

> After linking, restart the deployments so pods pick up the new pull secret:
> ```bash
> oc -n demo rollout restart deploy/frontend deploy/api deploy/worker
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

Example (use the exact tag you pushed, e.g. `0.1.0`):

```bash
helm upgrade --install microdemo ./microdemo-chart \
  --namespace demo \
  --set images.frontend=quay.io/YOUR_ORG/microdemo/frontend:0.1.0 \
  --set images.api=quay.io/YOUR_ORG/microdemo/api:0.1.0 \
  --set images.worker=quay.io/YOUR_ORG/microdemo/worker:0.1.0
```

> **Note:** If you only pushed a versioned tag (e.g. `0.1.0`) and the chart defaults to `:latest`,
> the pods will fail with `manifest unknown` / image not found. Always match the tag to what was actually pushed.

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

### Consul Connect sidecar injection

Consul Connect injection can be enabled at the **namespace level**, the **pod/deployment level**, or both, depending on how Consul was installed.

#### Option A – Namespace label (auto-inject all pods in the namespace)

```bash
oc label namespace demo consul.hashicorp.com/connect-inject=true --overwrite
```

#### Option B – Per-pod annotation (opt-in per workload)

Add the annotation to each Deployment's pod template (recommended when the namespace label alone is not honoured).

Example `values.yaml` snippet (if the chart supports a custom annotations map):

```yaml
frontend:
  podAnnotations:
    consul.hashicorp.com/connect-inject: "true"
api:
  podAnnotations:
    consul.hashicorp.com/connect-inject: "true"
worker:
  podAnnotations:
    consul.hashicorp.com/connect-inject: "true"
```

Or patch existing deployments directly (this automatically triggers a rollout restart for each):

```bash
for deploy in frontend api worker; do
  oc -n demo patch deploy/$deploy -p \
    '{"spec":{"template":{"metadata":{"annotations":{"consul.hashicorp.com/connect-inject":"true"}}}}}'
done
```

#### Verify injection

After pods restart, each should have **two** containers: the application and `consul-dataplane`:

```bash
oc -n demo get pods
# NAME                        READY   STATUS    ...
# frontend-xxx-yyy            2/2     Running   ...   ← 2/2 = injected
```

Check the containers in a specific pod:

```bash
oc -n demo get pod <POD_NAME> -o jsonpath='{.spec.containers[*].name}{"\n"}'
# expected: frontend consul-dataplane
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

### Consul ACL PermissionDenied

After enabling Consul Connect injection, pods may get stuck initialising and the `consul-dataplane` sidecar logs show:

```
rpc error: code = PermissionDenied desc = Permission denied
```

This means Consul's ACL system is enabled but the workload cannot obtain a service token / service identity.
The Consul installation must have Kubernetes auth configured and binding rules that map the pod's ServiceAccount
to a Consul ACL token with the appropriate service identity.

Exact remediation steps depend on **how** Consul is installed (Helm values, operator, CRDs, or CLI), but the common starting points are:

1. **Helm-managed Consul** – ensure `connectInject.enabled=true` and `global.acls.manageSystemACLs=true` in the Consul Helm values, then run `consul-k8s` ACL init.
2. **Operator/CRDs** – check that `AuthMethod`, `BindingRule`, and `ACLPolicy` CRs exist for the `demo` namespace.
3. **Inspect sidecar logs** to confirm the exact error:

```bash
# Get the consul-dataplane container logs for a stuck pod
oc -n demo logs <POD_NAME> -c consul-dataplane

# Check pod events for more context
oc -n demo describe pod <POD_NAME>
```

4. Verify the ServiceAccount token projection is present in the pod spec:

```bash
oc -n demo get pod <POD_NAME> -o jsonpath='{.spec.volumes}' | jq .
```

### Common deployment checks

Key commands used when debugging a fresh deployment:

```bash
# Image pull events – look for ErrImagePull / ImagePullBackOff
oc -n demo describe pod <POD_NAME>

# Confirm the image configured in each deployment
oc -n demo get deploy -o jsonpath='{range .items[*]}{.metadata.name}{": "}{.spec.template.spec.containers[0].image}{"\n"}{end}'

# Confirm imagePullSecrets on the ServiceAccount
oc -n demo get sa default -o jsonpath='{.imagePullSecrets}{"\n"}'

# Confirm imagePullSecrets on a specific pod
oc -n demo get pod <POD_NAME> -o jsonpath='{.spec.imagePullSecrets}{"\n"}'

# Watch rollout status
oc -n demo rollout status deploy/frontend
oc -n demo rollout status deploy/api
oc -n demo rollout status deploy/worker

# Verify the Route is admitted and get the hostname
oc -n demo get route
oc -n demo get route frontend -o jsonpath='{.spec.host}{"\n"}'
```

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
