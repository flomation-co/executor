// Package oracle_functions_pbf_listing_get fetches a single pre-built-function (PBF) listing by
// OCID and returns its identity, name, description and publisher.
package oracle_functions_pbf_listing_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Get PBF Listing"
	Description  = "Fetch a single pre-built-function (PBF) listing by OCID and return its name, description and publisher."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "pbf_listing_ocid", Type: core.ConnectionTypeString, Label: "PBF Listing OCID", Placeholder: "ocid1.fnpbflisting.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pbf_listing", Type: core.ConnectionTypeObject, Label: "PBF listing"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "PBF listing OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	pbfListingID, err := fn.RequiredString("pbf_listing_ocid", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	resp, err := client.GetPbfListing(fn.Context(), functions.GetPbfListingRequest{PbfListingId: &pbfListingID})
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}

	l := resp.PbfListing
	publisher := ""
	if l.PublisherDetails != nil {
		publisher = fn.Str(l.PublisherDetails.Name)
	}
	listing := map[string]interface{}{
		"id":              fn.Str(l.Id),
		"name":            fn.Str(l.Name),
		"description":     fn.Str(l.Description),
		"publisher":       publisher,
		"lifecycle_state": string(l.LifecycleState),
		"time_created":    fn.FormatTime(l.TimeCreated),
		"time_updated":    fn.FormatTime(l.TimeUpdated),
	}

	return fn.Result(fmt.Sprintf("Retrieved PBF listing %q", fn.Str(l.Name)), map[string]interface{}{
		"pbf_listing": listing,
		"id":          fn.Str(l.Id),
	}), nil
}
