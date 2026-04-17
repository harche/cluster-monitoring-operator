package operator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/openshift/cluster-monitoring-operator/pkg/manifests"
	"github.com/openshift/cluster-monitoring-operator/pkg/proposal"
)

// DefaultEligibleAlerts is the built-in set of CMO-managed alert names
// that can trigger proposals. Admins can modify this via
// alertProposals.includeAlerts / excludeAlerts in cluster-monitoring-config.
var DefaultEligibleAlerts = map[string]bool{
	"AlertmanagerFailedReload":               true,
	"AlertmanagerMembersInconsistent":        true,
	"AlertmanagerFailedToSendAlerts":         true,
	"AlertmanagerClusterFailedToSendAlerts":  true,
	"AlertmanagerConfigInconsistent":         true,
	"AlertmanagerClusterDown":                true,
	"PrometheusBadConfig":                    true,
	"PrometheusNotificationQueueRunningFull": true,
	"PrometheusNotConnectedToAlertmanagers":  true,
	"PrometheusTSDBReloadsFailing":           true,
	"PrometheusTSDBCompactionsFailing":       true,
	"PrometheusNotIngestingSamples":          true,
	"PrometheusRemoteStorageFailures":        true,
	"PrometheusRuleFailures":                 true,
	"PrometheusHighQueryLoad":                true,
	"ThanosQueryHttpRequestQueryErrorRateHigh": true,
	"ThanosQueryGrpcServerErrorRate":           true,
	"ThanosQueryOverload":                      true,
	"PrometheusOperatorSyncFailed":             true,
	"PrometheusOperatorNotReady":               true,
}

const (
	defaultMinFiringDuration    = 5 * time.Minute
	lightspeedWebhookPort       = 9099
	lightspeedWebhookPath       = "/api/v1/alerts"
	defaultAlertWorkflow        = "cmo-alert-remediation"
	defaultTriggerBootstrapFlow = "cmo-trigger-bootstrap"
)

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 2
	case "warning":
		return 1
	default:
		return 0
	}
}

func resolveEligibleAlerts(lc *manifests.LightspeedConfig) map[string]bool {
	eligible := make(map[string]bool, len(DefaultEligibleAlerts))
	for k, v := range DefaultEligibleAlerts {
		eligible[k] = v
	}
	if lc == nil || lc.AlertProposals == nil {
		return eligible
	}
	for _, name := range lc.AlertProposals.IncludeAlerts {
		eligible[name] = true
	}
	for _, name := range lc.AlertProposals.ExcludeAlerts {
		delete(eligible, name)
	}
	return eligible
}

func resolveAlertConfig(lc *manifests.LightspeedConfig) (minSeverity string, minFiring time.Duration) {
	minSeverity = "warning"
	minFiring = defaultMinFiringDuration
	if lc == nil || lc.AlertProposals == nil {
		return
	}
	if lc.AlertProposals.MinSeverity != "" {
		minSeverity = lc.AlertProposals.MinSeverity
	}
	if lc.AlertProposals.MinFiringDuration != "" {
		if d, err := time.ParseDuration(lc.AlertProposals.MinFiringDuration); err == nil {
			minFiring = d
		}
	}
	return
}

func alertProposalsEnabled(lc *manifests.LightspeedConfig) bool {
	if lc == nil {
		return true
	}
	if lc.AlertProposals != nil && lc.AlertProposals.Enabled != nil {
		return *lc.AlertProposals.Enabled
	}
	return true
}

// --- AlertManager webhook payload ---

type alertmanagerPayload struct {
	Status string            `json:"status"`
	Alerts []alertmanagerMsg `json:"alerts"`
}

type alertmanagerMsg struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	Fingerprint string            `json:"fingerprint"`
}

// --- Webhook handler ---

// lightspeedWebhookHandler receives AlertManager webhook payloads and creates
// LightspeedProposal CRs. Config is atomically swapped on each sync() so
// changes take effect without restart.
type lightspeedWebhookHandler struct {
	proposalCreator *proposal.Creator
	config          atomic.Pointer[manifests.LightspeedConfig]
}

func (h *lightspeedWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var payload alertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lc := h.config.Load()
	if !alertProposalsEnabled(lc) {
		w.WriteHeader(http.StatusOK)
		return
	}

	minSeverity, minFiring := resolveAlertConfig(lc)
	minSeverityRank := severityRank(minSeverity)
	eligibleAlerts := resolveEligibleAlerts(lc)

	ctx := r.Context()
	now := time.Now()
	created := 0

	for _, a := range payload.Alerts {
		if a.Status != "firing" {
			continue
		}

		alertName := a.Labels["alertname"]
		if alertName == "" || !eligibleAlerts[alertName] {
			continue
		}

		severity := a.Labels["severity"]
		if severityRank(severity) < minSeverityRank {
			continue
		}

		if !a.StartsAt.IsZero() && now.Sub(a.StartsAt) < minFiring {
			continue
		}

		if alertName == "TargetDown" {
			if ns := a.Labels["namespace"]; ns != "openshift-monitoring" {
				continue
			}
		}

		info := proposal.AlertInfo{
			Name:        alertName,
			Severity:    severity,
			Namespace:   a.Labels["namespace"],
			Pod:         a.Labels["pod"],
			Container:   a.Labels["container"],
			Description: a.Annotations["description"],
			RunbookURL:  a.Annotations["runbook_url"],
			Labels:      a.Labels,
		}

		h.proposalCreator.MaybeCreateAlertProposal(ctx, info)
		created++
	}

	klog.V(4).Infof("Lightspeed webhook: processed %d alerts, created/checked %d proposals", len(payload.Alerts), created)
	w.WriteHeader(http.StatusOK)
}

// --- Operator integration ---

// initLightspeed initializes the proposal creator.
func (o *Operator) initLightspeed(cfg *rest.Config) {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Warningf("Failed to create dynamic client for LightspeedProposal: %v", err)
		return
	}

	o.proposalCreator = proposal.NewCreator(dynamicClient, proposal.DefaultConfig())
	o.lightspeedEnabled = true
	klog.Info("Lightspeed proposal integration initialized")
}

// startLightspeedWebhook starts the webhook HTTP server that receives
// AlertManager notifications. The handler's config is updated atomically
// on each sync() via updateLightspeedConfig.
func (o *Operator) startLightspeedWebhook() {
	if o.proposalCreator == nil {
		return
	}

	o.lightspeedHandler = &lightspeedWebhookHandler{
		proposalCreator: o.proposalCreator,
	}

	mux := http.NewServeMux()
	mux.Handle(lightspeedWebhookPath, o.lightspeedHandler)

	addr := fmt.Sprintf(":%d", lightspeedWebhookPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		klog.Infof("Lightspeed alert webhook listening on %s%s", addr, lightspeedWebhookPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Warningf("Lightspeed webhook server error: %v", err)
		}
	}()
}

// updateLightspeedConfig atomically updates the webhook handler's config
// and creates bootstrap proposals for new custom triggers.
// Called from sync().
func (o *Operator) updateLightspeedConfig(cmc *manifests.ClusterMonitoringConfiguration) {
	if o.lightspeedHandler == nil {
		return
	}
	var lc *manifests.LightspeedConfig
	if cmc != nil && cmc.LightspeedConfig != nil {
		lc = cmc.LightspeedConfig
	} else {
		lc = &manifests.LightspeedConfig{}
	}
	o.lightspeedHandler.config.Store(lc)

	if lc.Enabled && o.proposalCreator != nil {
		o.reconcileCustomTriggers(lc)
	}
}

// reconcileCustomTriggers creates bootstrap proposals for custom triggers
// that don't yet have a corresponding proposal.
func (o *Operator) reconcileCustomTriggers(lc *manifests.LightspeedConfig) {
	for _, trigger := range lc.CustomTriggers {
		if trigger.Name == "" || trigger.Intent == "" {
			continue
		}
		workflow := trigger.Workflow
		if workflow == "" {
			workflow = defaultTriggerBootstrapFlow
		}
		o.proposalCreator.MaybeCreateTriggerProposal(
			trigger.Name,
			trigger.Intent,
			workflow,
		)
	}
}

func (o *Operator) shouldCreateLightspeedProposals() bool {
	// TODO: gate behind feature gate when ready for production
	return true
}
