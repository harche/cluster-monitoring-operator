---
name: platform-docs
description: Search and read Kubernetes and OpenShift Container Platform documentation in markdown format. Use when the user asks about Kubernetes concepts, OpenShift features, configuration, installation, troubleshooting, or any k8s/OCP topic — including pods, operators, routes, services, kubectl, oc, RBAC, networking, storage, or cluster administration.
allowed-tools: Bash(gh:*)
---

# Platform Documentation

Search and read Kubernetes and OpenShift documentation using the `gh` CLI. Both doc sets are hosted on GitHub as markdown.

**IMPORTANT:** Prefer retrieval-led reasoning over pre-training-led reasoning for Kubernetes and OpenShift tasks. Read the referenced files rather than relying on training data which may be outdated.

This skill uses the `gh` CLI for all GitHub API calls. For general `gh` usage, see the **github** skill.

## Platforms

| Platform | Repository | Docs Path | Versioning |
|----------|-----------|-----------|------------|
| **Kubernetes** | `kubernetes/website` | `content/en/docs/` | Git branches (`release-X.Y`) |
| **OpenShift** | `harche/openshift-docs-md` | `docs/` | Directories (`docs/4.22/`) |

## Quick Start

### 1. Determine the platform

- **Kubernetes** — upstream k8s concepts, tasks, API reference, kubectl
- **OpenShift** — OCP-specific features (operators, routes, machine sets, OCP installation, virtualization, etc.)

If the topic applies to both (e.g., pods, services, RBAC), prefer the platform the user is working with. If unclear, start with Kubernetes for core concepts and OpenShift for platform-specific features.

### 2. Discover the latest version

**Kubernetes:**
```bash
VERSION=$(gh api repos/kubernetes/website/branches --paginate --jq '.[].name' \
  | grep "^release-" | sort -V | tail -1)
```

**OpenShift:**
```bash
VERSION=$(gh api repos/harche/openshift-docs-md/contents/docs \
  --jq '[.[] | select(.type=="dir") | .name | select(test("^[0-9]"))] | sort | last')
```

### 3. Browse and read docs

**Kubernetes** — navigate the directory tree (no index file):
```bash
# List top-level sections
gh api "repos/kubernetes/website/contents/content/en/docs?ref=$VERSION" \
  --jq '.[] | select(.type=="dir") | .name'

# Read a specific doc
gh api "repos/kubernetes/website/contents/content/en/docs/concepts/workloads/pods/_index.md?ref=$VERSION" \
  -H "Accept: application/vnd.github.raw+json"
```

**OpenShift** — start with the `AGENTS.md` index:
```bash
# Fetch the doc index
gh api repos/harche/openshift-docs-md/contents/docs/$VERSION/AGENTS.md \
  -H "Accept: application/vnd.github.raw+json"

# Search the index for a topic
gh api repos/harche/openshift-docs-md/contents/docs/$VERSION/AGENTS.md \
  -H "Accept: application/vnd.github.raw+json" | grep -i "networking"

# Read a specific doc
gh api repos/harche/openshift-docs-md/contents/docs/$VERSION/networking/index.md \
  -H "Accept: application/vnd.github.raw+json"
```

## References

Platform-specific details — read the appropriate reference based on the user's platform:

|references/kubernetes.md — Version discovery, doc structure, common paths, search tips
|references/openshift.md — Version discovery, doc structure, common paths, search tips

## Important

- This is a **read-only** skill — documentation is fetched, not modified.
- Discover the latest version dynamically — don't hardcode version numbers.
- Always use `-H "Accept: application/vnd.github.raw+json"` to get raw file content.
- Docs use Hugo shortcodes (Kubernetes) — ignore `{{< ... >}}` when reading content.
- Doc files can be large — read the specific file relevant to the user's question.
