// Package infrastructure_kubernetes_cronjob_suspend pauses a CronJob by patching
// spec.suspend to true, so the controller stops creating new Jobs on the
// schedule.
//
// Suspend only stops FUTURE scheduled runs. A Job the CronJob has already
// started keeps running to completion — suspending is not a kill switch for
// in-flight work; delete the running Job (or its pods) for that.
package infrastructure_kubernetes_cronjob_suspend

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Suspend CronJob"
	Description  = "Pause a CronJob so it stops scheduling new Jobs. Jobs already running are left alone."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+pause"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the cronjob lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "CronJob", Placeholder: "The cronjob to suspend", Required: true},
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

	patch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{"suspend": true},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Patch(ctx, auth, kubernetes.CronJobs, namespace, name, patch, kubernetes.PatchStrategicMerge)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	return kubernetes.ObjectResult(obj, fmt.Sprintf("Suspended cronjob %s — no new jobs will be scheduled until it is resumed", name)), nil
}
