// Package infrastructure_kubernetes_pod_logs reads a container's log.
//
// The /log subresource is the one Kubernetes endpoint in this node that does not
// speak JSON: it streams the container's stdout as text/plain. So the response
// body is used verbatim rather than decoded, and only an *error* comes back as a
// Status object.
//
// It does NOT follow that the request should ask for text/plain. The API server
// negotiates content types against a fixed list — application/json,
// application/yaml, application/vnd.kubernetes.protobuf — and answers
// `Accept: text/plain` with 406 Not Acceptable, even though the log it would have
// returned is text/plain. `*/*` is what kubectl effectively sends, and what works.
package infrastructure_kubernetes_pod_logs

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Get Pod Logs"
	Description  = "Read the logs of a container in a pod — the last N lines, a recent time window, or the previous crashed instance."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+file-lines"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the pod runs in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Pod", Placeholder: "The pod to read logs from", Required: true},
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "Required only when the pod has more than one container"},
	{Name: "tail_lines", Type: core.ConnectionTypeInteger, Label: "Tail Lines", Placeholder: "Only the last N lines (default 200; blank for the whole log)"},
	{Name: "since_seconds", Type: core.ConnectionTypeInteger, Label: "Since (seconds)", Placeholder: "Only lines from the last N seconds"},
	{Name: "previous", Type: core.ConnectionTypeBoolean, Label: "Previous Instance", Placeholder: "Read the log of the previous, crashed container instead of the running one"},
	{Name: "timestamps", Type: core.ConnectionTypeBoolean, Label: "Include Timestamps", Placeholder: "Prefix every line with its RFC3339 timestamp"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Pod Name"},
	{Name: "logs", Type: core.ConnectionTypeText, Label: "Logs"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// defaultTailLines keeps an unbounded read from dragging a whole pod's history
// into a flow's output by accident. An operator who wants everything clears it.
const defaultTailLines = 200

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	namespace, err := kubernetes.RequiredString("namespace", inputs)
	if err != nil {
		return nil, err
	}
	name, err := kubernetes.RequiredString("name", inputs)
	if err != nil {
		return nil, err
	}

	path, err := kubernetes.Pods.SubPath(namespace, name, "log")
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	if container := kubernetes.OptionalString("container", inputs); container != "" {
		q.Set("container", container)
	}
	if tail, ok := kubernetes.OptionalInt("tail_lines", inputs); ok {
		if tail > 0 {
			q.Set("tailLines", strconv.Itoa(tail))
		}
	} else {
		q.Set("tailLines", strconv.Itoa(defaultTailLines))
	}
	if since, ok := kubernetes.OptionalInt("since_seconds", inputs); ok && since > 0 {
		q.Set("sinceSeconds", strconv.Itoa(since))
	}
	if kubernetes.BoolInput("previous", inputs) {
		q.Set("previous", "true")
	}
	if kubernetes.BoolInput("timestamps", inputs) {
		q.Set("timestamps", "true")
	}
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	// "*/*", not "text/plain" — see the package comment: the API server 406s on
	// an Accept it does not list, and text/plain is not on that list.
	resp, err := kubernetes.ExecuteAPI(ctx, auth, http.MethodGet, path, nil, "", "*/*")
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}
	if err := kubernetes.CheckResponse(resp); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	logs := string(resp.Body)
	summary := fmt.Sprintf("Read %d bytes of logs from pod %s", len(resp.Body), name)
	if resp.Truncated {
		summary = fmt.Sprintf("Read %d bytes of logs from pod %s — truncated at the size cap; "+
			"narrow the window with Tail Lines or Since", len(resp.Body), name)
	}

	return map[string]interface{}{
		"id":          name,
		"logs":        logs,
		"truncated":   resp.Truncated,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}
