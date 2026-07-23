// Package oracle_cloudguard_target_change_compartment would move a Cloud Guard target into a
// different compartment — but the OCI Cloud Guard API exposes no such operation. CloudGuardClient
// offers ChangeCompartment operations for data sources, detector recipes, managed lists, responder
// recipes, saved queries, security recipes and security zones, but there is no ChangeTargetCompartment,
// and UpdateTargetDetails carries no CompartmentId either. A target's compartment is fixed at
// creation. So this action validates its inputs and returns a soft error explaining the limitation
// and the recreate-in-target workaround, rather than silently doing nothing.
package oracle_cloudguard_target_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Change Target Compartment"
	Description  = "Attempt to move a Cloud Guard target to another compartment — reports that OCI provides no such operation for targets."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "target_ocid", Type: core.ConnectionTypeString, Label: "Target OCID", Placeholder: "ocid1.cloudguardtarget.oc1..aaaa… (the target to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where you want the target)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	targetID, err := cg.RequiredString("target_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	destination, err := cg.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}

	return cg.ErrorResult(fmt.Sprintf(
		"Cloud Guard targets cannot be moved between compartments: the Cloud Guard API has no change-compartment operation for targets (only data sources, detector recipes, managed lists, responder recipes, saved queries, security recipes and security zones can be moved), and a target's compartment is fixed at creation. "+
			"To place target %s in compartment %s, recreate it there — create a new target in the destination compartment with the same target resource and recipes, then delete the original.",
		targetID, destination,
	)), nil
}
