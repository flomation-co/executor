// Package oracle_compute_instance_get_all lists Compute instances in an OCI
// compartment, optionally filtered by display name or lifecycle state. It is the
// reference implementation for the Oracle Cloud Compute action template: an
// inline API-signing-key + scope input block, tool_result as the first output,
// and Execute delegating credential/client construction to the shared compute
// package.
package oracle_compute_instance_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	compute "flomation.app/automate/executor/actions/oracle/compute"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Compute: List Instances"
	Description  = "List Compute instances in an Oracle Cloud compartment, with their lifecycle state (running/stopped), shape, availability domain, region and tags. Optionally filter by display name or lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "20/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root compartment)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name filter", Placeholder: "Only instances with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State filter", Placeholder: "Filter by state (optional)", Options: []core.ConnectionOption{
		{Name: "Any (all states)", Value: ""},
		{Name: "Running", Value: "RUNNING"},
		{Name: "Stopped", Value: "STOPPED"},
		{Name: "Starting", Value: "STARTING"},
		{Name: "Stopping", Value: "STOPPING"},
		{Name: "Provisioning", Value: "PROVISIONING"},
		{Name: "Terminating", Value: "TERMINATING"},
		{Name: "Terminated", Value: "TERMINATED"},
		{Name: "Creating Image", Value: "CREATING_IMAGE"},
		{Name: "Moving", Value: "MOVING"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instances", Type: core.ConnectionTypeObject, Label: "Instances"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := compute.GetAuth(inputs)
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return compute.ErrorResult(err.Error()), nil
	}
	client, err := auth.ComputeClient()
	if err != nil {
		return compute.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := compute.Context()

	req := ocicore.ListInstancesRequest{CompartmentId: compute.StringPtr(compartment)}
	if dn := strings.TrimSpace(compute.OptionalString("display_name", inputs)); dn != "" {
		req.DisplayName = &dn
	}
	// The SDK's lifecycle filter is a typed enum, but OCI accepts the plain
	// upper-case string on the wire; pass it through so a new state Oracle adds
	// isn't gated by our build.
	if ls := strings.ToUpper(strings.TrimSpace(compute.OptionalString("lifecycle_state", inputs))); ls != "" {
		req.LifecycleState = ocicore.InstanceLifecycleStateEnum(ls)
	}

	var instances []map[string]interface{}
	for {
		resp, err := client.ListInstances(ctx, req)
		if err != nil {
			return compute.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			instances = append(instances, compute.SummariseInstance(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d instance(s) in the compartment", len(instances)),
		"instances":   instances,
		"count":       len(instances),
		"success":     true,
	}, nil
}
