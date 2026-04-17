# Lightspeed Proposal Integration

CMO creates `LightspeedProposal` CRs when monitoring alerts fire or when
admins define custom triggers. The proposals are consumed by OpenShift
Lightspeed (OLS), which runs an AI agent to diagnose the alert, query
Prometheus, and propose remediation steps.

## Architecture

```
cluster-monitoring-config (ConfigMap)
        |
        v
  CMO sync() ──> atomically updates webhook config + reconciles custom triggers
        |
        v
  AlertManager ── fires alert ──> CMO webhook (:9099/api/v1/alerts)
                                       |
                                       v
                              proposal.Creator.MaybeCreateAlertProposal()
                                       |
                                       v
                              LightspeedProposal CR (openshift-lightspeed)
                                       |
                                       v
                              OLS picks up proposal ──> runs OlsWorkflow
                                       |
                                       v
                              Agent diagnoses + proposes remediation
```

### Key components

| Component | Path | Role |
|-----------|------|------|
| Config types | `pkg/manifests/types.go` | `LightspeedConfig`, `AlertProposalConfig`, `CustomTrigger` structs |
| Config parsing | `pkg/manifests/config.go` | Unmarshals `lightspeed` section from `cluster-monitoring-config` |
| Webhook handler | `pkg/operator/lightspeed.go` | HTTP server on `:9099` receiving AlertManager payloads |
| Proposal creator | `pkg/proposal/proposal.go` | Builds and creates `LightspeedProposal` CRs via dynamic client |
| AlertManager route | `assets/alertmanager/secret.yaml` | Routes severity=critical|warning alerts to CMO webhook |
| Network policy | `assets/cluster-monitoring-operator/lightspeed-network-policy.yaml` | Restricts `:9099` to AlertManager pods only |
| Service | `assets/cluster-monitoring-operator/lightspeed-service.yaml` | Exposes webhook within the cluster |
| Prompts | `install/0000_50_cluster-monitoring-operator_50_lightspeed-prompts.yaml` | System prompt ConfigMaps for agents |
| Agents | `install/0000_50_cluster-monitoring-operator_51_lightspeed-agents.yaml` | OlsAgent CRs with structured output schemas |
| Workflows | `install/0000_50_cluster-monitoring-operator_52_lightspeed-workflows.yaml` | OlsWorkflow CRs for alert and trigger flows |
| RBAC | `install/0000_50_cluster-monitoring-operator_53_lightspeed-rbac.yaml` | ClusterRole/Binding for proposal + ConfigMap access |
| Skills | `lightspeed/skills/` | Agent skills: prometheus, monitoring-ops, platform-docs, redhat-support |

## Enabling Lightspeed proposals

Edit the `cluster-monitoring-config` ConfigMap in `openshift-monitoring`:

```bash
oc -n openshift-monitoring edit configmap cluster-monitoring-config
```

Add a `lightspeed` section under `config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    lightspeed:
      enabled: true
```

See `examples/lightspeed-cluster-monitoring-config.yaml` for a full example
with alert proposals and custom triggers.

## Alert-driven proposals

When `lightspeed.enabled: true`, CMO registers an AlertManager webhook
receiver. Firing alerts that match the eligible set create
`LightspeedProposal` CRs in `openshift-lightspeed`.

### Built-in eligible alerts (20)

These CMO-managed alerts trigger proposals by default:

- **Alertmanager**: FailedReload, MembersInconsistent, FailedToSendAlerts, ClusterFailedToSendAlerts, ConfigInconsistent, ClusterDown
- **Prometheus**: BadConfig, NotificationQueueRunningFull, NotConnectedToAlertmanagers, TSDBReloadsFailing, TSDBCompactionsFailing, NotIngestingSamples, RemoteStorageFailures, RuleFailures, HighQueryLoad
- **Thanos**: QueryHttpRequestQueryErrorRateHigh, QueryGrpcServerErrorRate, QueryOverload
- **Prometheus Operator**: SyncFailed, NotReady

### Configuration options

| Field | Default | Description |
|-------|---------|-------------|
| `alertProposals.enabled` | `true` | Toggle alert-driven proposals independently of custom triggers |
| `alertProposals.minSeverity` | `"warning"` | Minimum severity: `"critical"` or `"warning"` |
| `alertProposals.minFiringDuration` | `"5m"` | How long an alert must fire before a proposal is created |
| `alertProposals.includeAlerts` | `[]` | Additional alert names beyond the 20 built-in |
| `alertProposals.excludeAlerts` | `[]` | Remove specific built-in alerts from the eligible set |

`includeAlerts` accepts **any** Prometheus alert name -- it does not have to
be CMO-managed. When that alert fires and AlertManager pushes it to CMO's
webhook, CMO creates a `LightspeedProposal` and the agent investigates.

### Deduplication

Proposals use deterministic names (`cmo-alert-{sanitized-name}-{hash}`) and a
GET-before-CREATE pattern. The same alert firing multiple times will not create
duplicate proposals.

## Custom triggers

Custom triggers let admins describe a monitoring condition in plain English.
CMO creates a bootstrap `LightspeedProposal` that tells the AI agent to:

1. Discover relevant Prometheus metrics
2. Write and test PromQL expressions on the live cluster
3. Propose multiple alert rule options with different thresholds
4. After user approval, create the `PrometheusRule` CR

```yaml
lightspeed:
  enabled: true
  customTriggers:
    - name: etcd-latency
      intent: >-
        Alert when etcd fsync latency p99 exceeds 100ms for 10 minutes.
        This typically indicates disk pressure on the control plane nodes.
        Remediation should check disk I/O and suggest node scaling.
    - name: api-error-budget
      intent: >-
        Track the error budget for the Kubernetes API server.
        Alert when the remaining budget drops below 20% over a 30-day window.
```

## How it works end-to-end

1. Admin sets `lightspeed.enabled: true` in `cluster-monitoring-config`
2. CMO `sync()` atomically updates the webhook handler's config and creates
   bootstrap proposals for any `customTriggers`
3. A monitored alert fires (e.g., `PrometheusBadConfig`)
4. AlertManager sends a webhook POST to `CMO :9099/api/v1/alerts`
5. Webhook filters: must be firing, eligible, severity >= threshold, duration >= min
6. `proposal.Creator.MaybeCreateAlertProposal()` creates a `LightspeedProposal` CR
   in `openshift-lightspeed` with the `cmo-alert-remediation` workflow
7. OLS picks up the proposal, selects the `cmo-alert-advisor` agent
8. Agent uses skills (prometheus, monitoring-ops, platform-docs, redhat-support)
   to diagnose the alert and propose structured remediation

## OLS resources installed by CMO

| Resource | Name | Purpose |
|----------|------|---------|
| ConfigMap | `cmo-analysis-prompt` | System prompt for alert diagnosis |
| ConfigMap | `cmo-trigger-bootstrap-prompt` | System prompt for custom trigger translation |
| OlsAgent | `cmo-alert-advisor` | Agent with structured output schema for alert remediation |
| OlsAgent | `cmo-trigger-advisor` | Agent that proposes PrometheusRule options |
| OlsWorkflow | `cmo-alert-remediation` | Full analysis-execution-verification lifecycle |
| OlsWorkflow | `cmo-trigger-bootstrap` | Analysis-only (returns proposals, does not execute) |
| ClusterRole | `cluster-monitoring-operator-lightspeed` | RBAC: get/create LightspeedProposals, read prompt ConfigMaps |

## Environment variable overrides

These are read by `proposal.DefaultConfig()` and can be set on the CMO
deployment for testing:

| Variable | Default | Description |
|----------|---------|-------------|
| `LIGHTSPEED_PROPOSAL_NAMESPACE` | `openshift-lightspeed` | Namespace for LightspeedProposal CRs |
| `LIGHTSPEED_ALERT_WORKFLOW` | `cmo-alert-remediation` | OlsWorkflow for alert proposals |
| `LIGHTSPEED_PROMPT_CONFIGMAP` | `cmo-analysis-prompt` | ConfigMap holding the system prompt |

## Testing without a full OLS deployment

You can validate the integration by creating a mock `LightspeedProposal` CRD
and checking that CMO creates proposals. See
`examples/lightspeed-proposal-example.yaml` for the shape of a proposal CR.

```bash
# Apply the CRD (if OLS is not installed)
oc apply -f examples/lightspeed-proposal-crd-stub.yaml

# Enable lightspeed in CMO config
oc -n openshift-monitoring patch configmap cluster-monitoring-config \
  --type merge -p '{"data":{"config.yaml":"lightspeed:\n  enabled: true\n"}}'

# Wait for a built-in alert to fire, or manually trigger one
# Then check for proposals:
oc -n openshift-lightspeed get lightspeedproposals

# View proposal details:
oc -n openshift-lightspeed get lightspeedproposal cmo-alert-prometheusbadco-<hash> -o yaml
```
