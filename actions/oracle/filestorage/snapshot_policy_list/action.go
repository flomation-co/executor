// Package oracle_filestorage_snapshot_policy_list lists file system snapshot policies
// in a compartment + availability domain (both required — snapshot policies are AD-scoped).
package oracle_filestorage_snapshot_policy_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: List Snapshot Policies"
	Description  = "List the Oracle Cloud file system snapshot policies in a compartment and availability domain (both required — snapshot policies are AD-scoped), optionally filtered by display name. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+calendar"
	Date         = "22/07/2026"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only snapshot policies with this exact display name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot_policies", Type: core.ConnectionTypeObject, Label: "Snapshot Policies"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	ad, err := fss.RequiredAvailabilityDomain(inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	req := filestorage.ListFilesystemSnapshotPoliciesRequest{CompartmentId: &compartment, AvailabilityDomain: &ad}
	if v := strings.TrimSpace(fss.OptionalString("display_name", inputs)); v != "" {
		req.DisplayName = &v
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fss.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListFilesystemSnapshotPolicies(fss.Context(), req)
		if err != nil {
			return fss.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			p := &resp.Items[i]
			out = append(out, map[string]interface{}{
				"id":                  fss.Str(p.Id),
				"display_name":        fss.Str(p.DisplayName),
				"compartment_id":      fss.Str(p.CompartmentId),
				"availability_domain": fss.Str(p.AvailabilityDomain),
				"policy_prefix":       fss.Str(p.PolicyPrefix),
				"lifecycle_state":     string(p.LifecycleState),
				"time_created":        fss.FormatTime(p.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return fss.Result(fmt.Sprintf("Found %d snapshot policy(ies)", len(out)), map[string]interface{}{
		"snapshot_policies": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
