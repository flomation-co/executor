// Package infrastructure_kubernetes_event_list lists cluster Events — the
// "why will my pod not start?" diagnostic. Events carry the reason a scheduler
// could not place a pod, an image pull failed, a probe is failing, or a volume
// would not mount.
//
// These are core/v1 Events (/api/v1/.../events), not the newer events.k8s.io/v1
// kind. core/v1 is what a 1.23+ cluster always serves and is the shape kubectl
// reads by default, so it is the safe target across cluster versions.
//
// Events are ephemeral: the cluster garbage-collects them after roughly one hour
// by default (--event-ttl on the API server). A diagnostic that runs too long
// after the incident will find nothing — that is the cluster's retention, not a
// bug here.
//
// The Involved Object filter is a convenience over the field selector. Kubernetes
// selects an object's events with involvedObject.name=<name>, and ANDs
// comma-separated field selectors — so when the operator also supplies their own
// Field Selector, the two are joined with a comma rather than one overwriting the
// other.
package infrastructure_kubernetes_event_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes List Events"
	Description  = "List cluster Events in a namespace or across all of them — the diagnostic for why a pod will not start, an image will not pull, or a volume will not mount."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+list"
	Date         = "10/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_server_url", Type: core.ConnectionTypeString, Label: "API Server URL", Placeholder: "https://your-cluster:6443 — the Kubernetes API endpoint", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "Service Account Token", Value: "token"},
		{Name: "Client Certificate (mTLS)", Value: "client_cert"},
		{Name: "Kubeconfig (paste)", Value: "kubeconfig"},
	}},
	{Name: "service_account_token", Type: core.ConnectionTypeSecret, Label: "Service Account Token", Placeholder: "kubectl create token <serviceaccount> -n <namespace>", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "cluster_ca_cert", Type: core.ConnectionTypeText, Label: "Cluster CA Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE----- … Leave blank to use the system trust store", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token", "client_cert"}}},
	{Name: "client_certificate", Type: core.ConnectionTypeSecret, Label: "Client Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE-----", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"client_cert"}}},
	{Name: "client_key", Type: core.ConnectionTypeSecret, Label: "Client Key (PEM)", Placeholder: "-----BEGIN RSA PRIVATE KEY-----", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"client_cert"}}},
	{Name: "kubeconfig", Type: core.ConnectionTypeSecret, Label: "Kubeconfig YAML", Placeholder: "Paste the full kubeconfig; the current-context is used", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"kubeconfig"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip API server certificate verification — only for self-signed clusters with no CA to hand"},

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "Leave blank to list events across every namespace"},
	{Name: "involved_object", Type: core.ConnectionTypeString, Label: "Involved Object", Placeholder: "Only events about this object, by name (e.g. the pod that won't start)"},
	{Name: "field_selector", Type: core.ConnectionTypeString, Label: "Field Selector", Placeholder: "reason=FailedScheduling,type=Warning"},
	{Name: "label_selector", Type: core.ConnectionTypeString, Label: "Label Selector", Placeholder: "Rarely useful for Events — most carry no labels"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Objects per page (default 100, max 500)"},
	{Name: "continue_token", Type: core.ConnectionTypeString, Label: "Continue Token", Placeholder: "Resume from a previous run's continue token"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow continue tokens until every event is fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Events"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "continue_token", Type: core.ConnectionTypeString, Label: "Continue Token"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "More Available"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	namespace := kubernetes.OptionalString("namespace", inputs)
	opts := kubernetes.ListOptionsFrom(inputs)

	// Fold the Involved Object convenience into the field selector. Kubernetes
	// ANDs comma-separated field selectors, so an operator-supplied selector is
	// preserved by joining rather than replaced.
	if involved := kubernetes.OptionalString("involved_object", inputs); involved != "" {
		clause := "involvedObject.name=" + involved
		if opts.FieldSelector != "" {
			opts.FieldSelector += "," + clause
		} else {
			opts.FieldSelector = clause
		}
	}

	items, continueToken, pages, err := kubernetes.List(ctx, auth, kubernetes.Events, namespace, opts)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	scope := "every namespace"
	if namespace != "" {
		scope = "namespace " + namespace
	}

	summary := fmt.Sprintf("Found %d event(s) in %s", len(items), scope)
	if opts.ReturnAll && pages >= kubernetes.MaxAllPages && continueToken != "" {
		summary = fmt.Sprintf("Fetched %d event(s) across %d page(s) in %s; stopped at the %d-page safety cap — "+
			"narrow the filters, or pass the continue token back in to resume", len(items), pages, scope, kubernetes.MaxAllPages)
	} else if continueToken != "" {
		summary = fmt.Sprintf("Found %d event(s) in %s; more remain — pass the continue token back in to fetch the next page", len(items), scope)
	}

	return kubernetes.ListResult(items, continueToken, summary), nil
}
