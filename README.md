<p align="center">
  <img src="https://recif-platform.github.io/logo.png" alt="Recif Operator" width="80" />
</p>

<h1 align="center">Recif Operator</h1>

<p align="center">
  <strong>Kubernetes operator that turns declarative Agent specs into running containers.</strong>
</p>

<p align="center">
  <a href="https://github.com/recif-platform/recif-operator/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache_2.0-blue?style=flat-square" alt="License" /></a>
  <a href="https://github.com/recif-platform/recif-operator/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/recif-platform/recif-operator/ci.yml?style=flat-square&label=CI" alt="CI" /></a>
  <img src="https://img.shields.io/badge/version-v0.2.0-green?style=flat-square" alt="Version" />
  <a href="https://discord.gg/P279TT4ZCp"><img src="https://img.shields.io/badge/Discord-Join-5865F2?style=flat-square&logo=discord&logoColor=white" alt="Discord" /></a>
</p>

---

The Recif Operator is a [kubebuilder](https://book.kubebuilder.io)-based Kubernetes operator that reconciles `Agent` and `Tool` custom resources into fully wired Deployments, Services, and ConfigMaps. You declare what an agent should be -- model, tools, knowledge bases, replicas -- and the operator ensures it runs. It is the infrastructure layer of the [Recif platform](https://github.com/recif-platform).

**Group:** `agents.recif.dev` | **Version:** `v1` | **Kinds:** `Agent`, `Tool`

---

## Quick Start

### Prerequisites

Go 1.22+, kubectl, access to a Kubernetes cluster.

### Install CRDs and run

```bash
# Install CRDs into the cluster
make install

# Run the operator locally (outside the cluster)
make run
```

### Build and deploy as a container

```bash
make docker-build IMG=ghcr.io/recif-platform/recif-operator:latest
make deploy IMG=ghcr.io/recif-platform/recif-operator:latest
```

### Create an agent

```yaml
apiVersion: agents.recif.dev/v1
kind: Agent
metadata:
  name: my-agent
  namespace: team-default
spec:
  name: my-agent
  framework: adk
  modelType: vertex-ai
  modelId: gemini-2.5-flash
  systemPrompt: "You are a helpful assistant."
  image: ghcr.io/recif-platform/corail:latest
  replicas: 1
  tools:
    - web-search
  knowledgeBases:
    - product-docs
```

```bash
kubectl apply -f agent.yaml
kubectl get agents -n team-default
```

---

## Key Features

- **Agent CRD** -- Declarative agent specs with model, strategy, tools, knowledge bases, replicas, and system prompt. The single source of truth.
- **Tool CRD** -- Separate tool definitions (HTTP, CLI, MCP, built-in) referenced by name from agents. Resolved at reconcile time.
- **Auto-reconciliation** -- Watches Agent CRDs and creates/updates Deployments, Services, and ConfigMaps. Owner references ensure garbage collection.
- **Config-hash rolling restarts** -- SHA-256 hash of ConfigMap data annotated on pods. Any config change triggers a zero-downtime rolling restart.
- **GCP service account isolation** -- Per-agent credential mounting. Sets `GOOGLE_APPLICATION_CREDENTIALS` and `GOOGLE_CLOUD_PROJECT` automatically.
- **Canary deployment support** -- Agent CRD includes a `canary` spec for progressive traffic shifting (image, model, prompt overrides).
- **Health probes** -- Liveness and readiness probes configured on every agent pod automatically.
- **Leader election** -- HA-safe: only one operator instance reconciles at a time.

---

## Architecture

```
+------------------------------------------------------------------+
|                     Kubernetes Cluster                            |
|                                                                  |
|   +-------------------+                                          |
|   | Recif Operator    |                                          |
|   | (this repo)       |                                          |
|   +---------+---------+                                          |
|             |                                                    |
|             | watches                                            |
|             v                                                    |
|   +-------------------+    +-------------------+                 |
|   | Agent CRDs        |    | Tool CRDs         |                |
|   | agents.recif.dev  |    | agents.recif.dev  |                |
|   +---------+---------+    +---------+---------+                 |
|             |                        |                           |
|             | resolves tools,        |                           |
|             | builds config          |                           |
|             v                        |                           |
|   +---------+---------+              |                           |
|   |   Reconciler      |<-------------+                           |
|   +---------+---------+                                          |
|             |                                                    |
|             | creates / updates                                  |
|             |                                                    |
|   +---------+---------+---------+                                |
|   v                   v         v                                |
|   +------------+ +----------+ +-------------+                    |
|   | Deployment | | Service  | | ConfigMap   |                    |
|   | (Corail    | | :8000    | | (all CORAIL |                    |
|   |  container)| | :8001    | |  env vars)  |                    |
|   +------------+ +----------+ +-------------+                    |
|        |                                                         |
|        | ownerRef --> Agent CRD                                  |
|        | (delete agent = delete everything)                      |
+------------------------------------------------------------------+
```

---

## CRDs

### Agent CRD

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | required | Agent name |
| `framework` | enum | required | Runtime: `adk`, `langchain`, `crewai`, `autogen`, `custom` |
| `modelType` | string | `stub` | LLM provider |
| `modelId` | string | `stub-echo` | Model identifier |
| `strategy` | string | `simple` | Agent strategy: `simple`, `rag`, `react` |
| `systemPrompt` | string | `""` | System prompt |
| `image` | string | `corail:latest` | Container image |
| `replicas` | int32 | `1` | Pod replicas (0-10) |
| `tools[]` | []string | `[]` | Tool CRD names to resolve |
| `knowledgeBases[]` | []string | `[]` | Knowledge base IDs for RAG |
| `gcpServiceAccount` | string | `""` | GCP SA email for credential mounting |
| `canary` | CanarySpec | nil | Canary deployment config |

### Tool CRD

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Tool name |
| `type` | enum | `http`, `cli`, `mcp`, `builtin` |
| `endpoint` | string | HTTP endpoint URL |
| `mcpEndpoint` | string | MCP server endpoint |
| `binary` | string | CLI binary path |
| `enabled` | bool | Whether the tool is active |

---

## Make Targets

| Target | Description |
|--------|-------------|
| `make install` | Install CRDs into the cluster |
| `make uninstall` | Remove CRDs from the cluster |
| `make run` | Run the operator locally |
| `make build` | Build the manager binary |
| `make docker-build` | Build the operator container image |
| `make deploy` | Deploy the operator to the cluster |
| `make undeploy` | Remove the operator from the cluster |
| `make manifests` | Regenerate CRD and RBAC manifests |
| `make generate` | Regenerate DeepCopy methods |
| `make test` | Run unit tests |
| `make lint` | Run golangci-lint |

---

## Related Repositories

| Repository | Description |
|------------|-------------|
| [recif](https://github.com/recif-platform/recif) | Go API + Next.js dashboard -- the control tower |
| [corail](https://github.com/recif-platform/corail) | Python agent runtime -- the engine inside every agent pod |
| [helm-charts](https://github.com/recif-platform/helm-charts) | Helm chart for one-command platform installation |

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/your-feature`)
3. Write tests for new functionality
4. Ensure `make lint` and `make test` pass
5. Submit a pull request

---

## Links

- [Documentation](https://recif-platform.github.io/docs)
- [Discord](https://discord.gg/P279TT4ZCp)
- [GitHub Organization](https://github.com/recif-platform)

---

## License

[Apache License 2.0](LICENSE) -- Copyright 2026 Sciences44.
