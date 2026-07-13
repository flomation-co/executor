// Package infrastructure_kubernetes_service_create creates a Service that
// exposes a set of pods on a stable virtual IP and port.
//
// The inputs are the flat, common case: one port, one protocol, a label
// selector, and a Service type. A Service with several ports or session affinity
// needs a shape a handful of flat fields cannot express — reach for
// apply_manifest there.
//
// An ExternalName Service is a different object entirely, and the fields switch
// with it. It has no selector, no ports and no virtual IP: it is a CNAME record,
// mapping an in-cluster DNS name onto an external hostname (a managed database,
// say). Kubernetes rejects a ports array on one, and requires spec.externalName.
// So the type dropdown drives which inputs the editor shows, and Execute builds
// one of two bodies.
//
// A few Service semantics the fields do not spell out:
//   - target_port is the port the pods actually listen on; it defaults to the
//     service port, which is the common case where they are the same number.
//   - node_port only means anything for a NodePort (or LoadBalancer) service; on
//     a ClusterIP service the API server rejects it. It is left unset unless the
//     operator supplies one, so the cluster allocates a free port itself.
package infrastructure_kubernetes_service_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create Service"
	Description  = "Create a Service to expose a set of pods on a stable IP and port — ClusterIP, NodePort, LoadBalancer, or ExternalName."
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to create the service in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "A DNS-safe name, e.g. web", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Options: []core.ConnectionOption{
		{Name: "ClusterIP", Value: "ClusterIP"},
		{Name: "NodePort", Value: "NodePort"},
		{Name: "LoadBalancer", Value: "LoadBalancer"},
		{Name: "ExternalName", Value: "ExternalName"},
	}},
	{Name: "external_name", Type: core.ConnectionTypeString, Label: "External Hostname", Placeholder: "db.example.com — the hostname this service resolves to", Visible: &core.VisibleWhen{Field: "type", Values: []string{"ExternalName"}}},
	{Name: "selector", Type: core.ConnectionTypeObject, Label: "Selector", Placeholder: `The label map choosing the pods, e.g. {"app":"web"}`, Visible: &core.VisibleWhen{Field: "type", Values: []string{"", "ClusterIP", "NodePort", "LoadBalancer"}}},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "The port the service exposes, e.g. 80", Visible: &core.VisibleWhen{Field: "type", Values: []string{"", "ClusterIP", "NodePort", "LoadBalancer"}}},
	{Name: "target_port", Type: core.ConnectionTypeInteger, Label: "Target Port", Placeholder: "The port the pods listen on (defaults to Port)", Visible: &core.VisibleWhen{Field: "type", Values: []string{"", "ClusterIP", "NodePort", "LoadBalancer"}}},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Options: []core.ConnectionOption{
		{Name: "TCP", Value: "TCP"},
		{Name: "UDP", Value: "UDP"},
	}, Visible: &core.VisibleWhen{Field: "type", Values: []string{"", "ClusterIP", "NodePort", "LoadBalancer"}}},
	{Name: "node_port", Type: core.ConnectionTypeInteger, Label: "Node Port", Placeholder: "Only for NodePort/LoadBalancer — leave blank to let the cluster pick (30000–32767)", Visible: &core.VisibleWhen{Field: "type", Values: []string{"NodePort", "LoadBalancer"}}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Service Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Service"},
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

	svcType := kubernetes.OptionalString("type", inputs)
	if svcType == "" {
		svcType = "ClusterIP"
	}

	var spec map[string]interface{}

	var summary string

	if svcType == "ExternalName" {
		// A CNAME, not a virtual IP. Kubernetes requires spec.externalName and
		// rejects a ports array, so none of the pod-facing fields apply.
		externalName, err := kubernetes.RequiredString("external_name", inputs)
		if err != nil {
			return nil, fmt.Errorf("external_name is required for an ExternalName service — the hostname it should resolve to, e.g. db.example.com")
		}
		spec = map[string]interface{}{
			"type":         svcType,
			"externalName": externalName,
		}
		summary = "Created ExternalName service " + name + " in namespace " + namespace + ", resolving to " + externalName
	} else {
		selector, err := kubernetes.StringMapInput("selector", inputs)
		if err != nil {
			return nil, err
		}
		if len(selector) == 0 {
			return nil, fmt.Errorf(`selector is required — provide the label map that chooses the pods, e.g. {"app":"web"}`)
		}

		port, ok := kubernetes.OptionalInt("port", inputs)
		if !ok {
			return nil, fmt.Errorf("port is required")
		}

		targetPort := port
		if tp, ok := kubernetes.OptionalInt("target_port", inputs); ok {
			targetPort = tp
		}

		protocol := kubernetes.OptionalString("protocol", inputs)
		if protocol == "" {
			protocol = "TCP"
		}

		portEntry := map[string]interface{}{
			"port":       port,
			"targetPort": targetPort,
			"protocol":   protocol,
		}
		// nodePort is only valid on NodePort/LoadBalancer services; omit it unless
		// the operator pinned one, so the cluster allocates a free port itself.
		if nodePort, ok := kubernetes.OptionalInt("node_port", inputs); ok {
			portEntry["nodePort"] = nodePort
		}

		spec = map[string]interface{}{
			"type":     svcType,
			"selector": selector,
			"ports":    []interface{}{portEntry},
		}
		summary = fmt.Sprintf("Created %s service %s in namespace %s on port %d", svcType, name, namespace, port)
	}

	body := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]interface{}{"name": name},
		"spec":       spec,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Create(ctx, auth, kubernetes.Services, namespace, body)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	return kubernetes.ObjectResult(obj, summary), nil
}
