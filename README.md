<div align="center">

# :gear: Récif Operator

**Kubernetes operator that turns declarative Agent specs into running containers.**

[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Kubernetes 1.28+](https://img.shields.io/badge/Kubernetes-1.28+-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Built with Kubebuilder](https://img.shields.io/badge/Built_with-Kubebuilder_4-blue)](https://book.kubebuilder.io)
[![License Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-orange)](LICENSE)
[![Discord](https://img.shields.io/badge/Discord-Join-5865F2?logo=discord&logoColor=white)](https://discord.gg/P279TT4ZCp)

</div>

> **⚠️ Alpha Release** — Récif has just been open-sourced. The platform is functional but expect rough edges, breaking changes, and evolving APIs. We're actively looking for **contributors** — whether you're into Go, Python, Kubernetes, React, or AI/ML. Come shape the future of agent operations with us.
>
> **[Join us on Discord →](https://discord.gg/P279TT4ZCp)** · **[Documentation →](https://recif-platform.github.io/docs/introduction)**

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [CRDs](#crds)
  - [Agent CRD](#agent-crd)
  - [Tool CRD](#tool-crd)
- [Quick Start](#quick-start)
- [Example Agent](#example-agent)
- [Key Behaviors](#key-behaviors)
- [Make Targets](#make-targets)
- [License](#license)

---

## Overview

The Récif Operator is a [kubebuilder](https://book.kubebuilder.io)-based Kubernetes operator that reconciles `Agent` custom resources into fully wired Deployments, Services, and ConfigMaps. It is the infrastructure layer of the Récif platform — you declare *what* an agent should be, and the operator ensures it runs.

**Group:** `agents.recif.dev` &nbsp;|&nbsp; **Version:** `v1` &nbsp;|&nbsp; **Kinds:** `Agent`, `Tool`

---

## Architecture

```mermaid
flowchart LR
    A["Agent CRD<br/><code>agents.recif.dev/v1</code>"] -->|watch| O["Récif Operator"]
    T["Tool CRDs"] -->|resolve| O
    O -->|create/update| D["Deployment"]
    O -->|create/update| S["Service"]
    O -->|create/update| C["ConfigMap"]
    D -.->|ownerRef| A
    S -.->|ownerRef| A
    C -.->|ownerRef| A

    style A fill:#0d1b2a,stroke:#00b4d8,color:#e0e0e0
    style O fill:#0d1b2a,stroke:#f77f00,color:#e0e0e0
    style T fill:#0d1b2a,stroke:#00b4d8,color:#e0e0e0
    style D fill:#0d1b2a,stroke:#06d6a0,color:#e0e0e0
    style S fill:#0d1b2a,stroke:#06d6a0,color:#e0e0e0
    style C fill:#0d1b2a,stroke:#06d6a0,color:#e0e0e0
```

**Reconciliation loop:**

1. Operator watches `Agent` resources.
2. For each Agent, it resolves Tool CRDs and knowledge bases.
3. Builds a ConfigMap with all Corail environment variables.
4. Creates/updates a Deployment (with config-hash annotation for rolling restarts).
5. Creates a ClusterIP Service exposing ports `8000` (HTTP) and `8001` (control).
6. Sets owner references on all child resources — delete the Agent, everything cleans up.

---

## CRDs

### Agent CRD

<details>
<summary><strong>Full spec reference</strong> — click to expand</summary>

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | `string` | *required* | Agent name |
| `framework` | `enum` | *required* | Runtime framework: `adk`, `langchain`, `crewai`, `autogen`, `custom` |
| `strategy` | `string` | `simple` | Agent strategy (e.g. `simple`, `rag`, `react`) |
| `channel` | `string` | `rest` | Communication channel |
| `modelType` | `string` | `stub` | Model provider: `openai`, `vertex-ai`, `google-ai`, `stub` |
| `modelId` | `string` | `stub-echo` | Model identifier |
| `systemPrompt` | `string` | `""` | System prompt injected into the agent |
| `storage` | `enum` | `memory` | State storage: `memory` or `postgresql` |
| `databaseUrl` | `string` | `""` | PostgreSQL connection string (when storage=postgresql) |
| `image` | `string` | `corail:latest` | Container image for the agent |
| `replicas` | `int32` | `1` | Number of pod replicas (0-10) |
| `tools[]` | `[]string` | `[]` | Tool CRD names to resolve and inject |
| `knowledgeBases[]` | `[]string` | `[]` | Knowledge base IDs for RAG |
| `skills[]` | `[]string` | `[]` | Skill IDs assigned to the agent |
| `envSecrets[]` | `[]string` | `["agent-env"]` | Secret names injected as env vars |
| `credentialSecret` | `string` | `gcp-adc` | Secret containing cloud credentials |
| `gcpServiceAccount` | `string` | `""` | GCP SA email; mounts `{agent}-gcp-sa` secret, sets `GOOGLE_APPLICATION_CREDENTIALS` and `GOOGLE_CLOUD_PROJECT` |
| `suggestionsProvider` | `string` | `""` | `llm` (dynamic) or `static` |
| `suggestions` | `string` | `""` | Static suggestions (JSON array) |
| `evalSampleRate` | `int32` | `0` | % of traces to auto-evaluate (0-100) |
| `judgeModel` | `string` | `""` | LLM-judge model for eval (e.g. `openai:/gpt-4o-mini`) |
| `canary` | `CanarySpec` | `nil` | Canary deployment config |

**CanarySpec:**

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | `bool` | Enable canary deployment |
| `weight` | `int32` | Traffic weight for canary (%) |
| `image` | `string` | Override image for canary |
| `modelType` | `string` | Override model type |
| `modelId` | `string` | Override model ID |
| `systemPrompt` | `string` | Override system prompt |
| `skills[]` | `[]string` | Override skills |
| `tools[]` | `[]string` | Override tools |
| `version` | `string` | Canary version label |

**Status fields:** `phase` (Pending/Running/Failed/Terminated), `replicas`, `endpoint`, `conditions[]`

</details>

### Tool CRD

Tools are declared as separate CRDs and referenced by name from `Agent.spec.tools[]`. The operator resolves them at reconcile time and serializes the full configuration as JSON into the agent's ConfigMap.

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Tool name |
| `type` | `enum` | `http`, `cli`, `mcp`, `builtin` |
| `category` | `string` | Tool category (default: `general`) |
| `endpoint` | `string` | HTTP endpoint URL |
| `mcpEndpoint` | `string` | MCP server endpoint |
| `binary` | `string` | CLI binary path |
| `enabled` | `bool` | Whether the tool is active (default: `true`) |

---

## Quick Start

**Prerequisites:** Go 1.22+, kubectl, access to a Kubernetes cluster.

```bash
# Install CRDs into the cluster
make install

# Run the operator locally (outside the cluster)
make run

# — or build and deploy as a container —
make docker-build IMG=recif-operator:dev
make deploy IMG=recif-operator:dev
```

---

## Example Agent

```yaml
apiVersion: agents.recif.dev/v1
kind: Agent
metadata:
  name: support-agent
  namespace: recif-agents
spec:
  name: support-agent
  framework: adk
  strategy: rag
  channel: rest
  modelType: vertex-ai
  modelId: gemini-2.0-flash
  systemPrompt: "You are a helpful customer support agent."
  image: europe-west1-docker.pkg.dev/my-project/recif/corail:latest
  replicas: 2
  storage: postgresql
  databaseUrl: postgresql://recif:recif@recif-postgresql:5432/corail_storage
  tools:
    - web-search
    - ticket-lookup
  knowledgeBases:
    - product-docs
  skills:
    - summarize
    - translate
  gcpServiceAccount: support-agent@my-project.iam.gserviceaccount.com
  evalSampleRate: 10
  judgeModel: openai:/gpt-4o-mini
  suggestionsProvider: llm
```

Apply it:

```bash
kubectl apply -f agent.yaml
kubectl get agents -n recif-agents
```

Expected output:

```
NAME            PHASE     REPLICAS   ENDPOINT                                                         AGE
support-agent   Running   2          http://support-agent.recif-agents.svc.cluster.local:8000          12s
```

---

## Key Behaviors

| Behavior | Detail |
|----------|--------|
| **Owner references** | All child resources (Deployment, Service, ConfigMap) have `ownerReferences` pointing back to the Agent. Deleting the Agent garbage-collects everything. |
| **Config hash restarts** | A SHA-256 hash of the ConfigMap data is set as the annotation `recif.dev/config-hash` on the pod template. Any config change triggers a rolling restart. |
| **Tool CRD resolution** | Tool names in `spec.tools[]` are looked up as `Tool` CRDs in the same namespace. Missing CRDs are treated as builtins with a warning in operator logs. |
| **Image pull policy** | Local images (no `/` in name) use `PullNever`; registry images use `PullIfNotPresent`. |
| **GCP SA isolation** | When `spec.gcpServiceAccount` is set, the operator mounts a per-agent secret `{agent}-gcp-sa` and injects `GOOGLE_APPLICATION_CREDENTIALS` + `GOOGLE_CLOUD_PROJECT`. |
| **Leader election** | Enabled for HA — only one operator instance reconciles at a time. |
| **Health probes** | Liveness (`/healthz`, 10s period) and readiness (`/healthz`, 5s period) are configured on every agent pod. |

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
| `make test-e2e` | Run end-to-end tests (Kind) |
| `make lint` | Run golangci-lint |

---

## License

Copyright 2026 Sciences44.

Licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0).
