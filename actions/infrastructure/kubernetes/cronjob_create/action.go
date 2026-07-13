// Package infrastructure_kubernetes_cronjob_create creates a CronJob (batch/v1)
// from a schedule, an image, and an optional command — the workflow equivalent
// of `kubectl create cronjob NAME --image=IMG --schedule="..." -- CMD`.
//
// The single container it builds takes the CronJob's own name, matching kubectl.
// The command line is tokenised with kubernetes.SplitCommand, so a quoted
// "sh -c 'do a thing'" survives as the argv the container is exec'd with rather
// than being naively space-split.
package infrastructure_kubernetes_cronjob_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create CronJob"
	Description  = "Create a CronJob that runs a container on a cron schedule — set the image, an optional command, and the concurrency policy."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+plus"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to create the cronjob in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "nightly-backup", Required: true},
	{Name: "schedule", Type: core.ConnectionTypeString, Label: "Schedule (cron)", Placeholder: "*/5 * * * * — standard five-field cron", Required: true},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image", Placeholder: "busybox:latest", Required: true},
	{Name: "command", Type: core.ConnectionTypeString, Label: "Command", Placeholder: `Optional override, e.g. sh -c 'echo hello'. Leave blank to use the image's entrypoint`},
	{Name: "suspend", Type: core.ConnectionTypeBoolean, Label: "Suspend", Placeholder: "Create it suspended — scheduled but not firing until you resume it"},
	{Name: "restart_policy", Type: core.ConnectionTypeString, Label: "Restart Policy", Options: []core.ConnectionOption{
		{Name: "OnFailure", Value: "OnFailure"},
		{Name: "Never", Value: "Never"},
	}},
	{Name: "concurrency_policy", Type: core.ConnectionTypeString, Label: "Concurrency Policy", Options: []core.ConnectionOption{
		{Name: "Allow", Value: "Allow"},
		{Name: "Forbid", Value: "Forbid"},
		{Name: "Replace", Value: "Replace"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "CronJob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "CronJob"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

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
	schedule, err := kubernetes.RequiredString("schedule", inputs)
	if err != nil {
		return nil, err
	}
	image, err := kubernetes.RequiredString("image", inputs)
	if err != nil {
		return nil, err
	}

	restartPolicy := kubernetes.OptionalString("restart_policy", inputs)
	if restartPolicy == "" {
		restartPolicy = "OnFailure"
	}
	concurrencyPolicy := kubernetes.OptionalString("concurrency_policy", inputs)
	if concurrencyPolicy == "" {
		concurrencyPolicy = "Allow"
	}

	// The container carries the CronJob's own name, matching `kubectl create
	// cronjob`.
	container := map[string]interface{}{
		"name":  name,
		"image": image,
	}
	if cmd := kubernetes.OptionalString("command", inputs); cmd != "" {
		args, err := kubernetes.SplitCommand(cmd)
		if err != nil {
			return nil, fmt.Errorf("command is not a valid shell command line: %w", err)
		}
		if len(args) > 0 {
			container["command"] = args
		}
	}

	spec := map[string]interface{}{
		"schedule":          schedule,
		"concurrencyPolicy": concurrencyPolicy,
		"jobTemplate": map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"restartPolicy": restartPolicy,
						"containers":    []interface{}{container},
					},
				},
			},
		},
	}
	// Only stamp suspend when the operator asked for it; leaving it out lets the
	// Kubernetes default (false) stand and keeps the object minimal.
	if kubernetes.BoolInput("suspend", inputs) {
		spec["suspend"] = true
	}

	body := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]interface{}{"name": name},
		"spec":       spec,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Create(ctx, auth, kubernetes.CronJobs, namespace, body)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(obj, fmt.Sprintf("Created cronjob %s on schedule %q in namespace %s", name, schedule, namespace))
	if kubernetes.ObjectName(obj) == "" {
		out["id"] = name
	}
	return out, nil
}
