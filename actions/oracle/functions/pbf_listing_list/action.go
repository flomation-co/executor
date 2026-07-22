// Package oracle_functions_pbf_listing_list lists the Pre-built Function (PBF) listings in the
// global Oracle Cloud catalog. PBF listings are tenancy-independent, so no compartment scopes the
// query; optional name / name-contains / id filters narrow it. Walks pagination up to a safe cap.
package oracle_functions_pbf_listing_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: List PBF Listings"
	Description  = "List the Pre-built Function (PBF) listings in the Oracle Cloud catalog, with optional name, name-contains and id filters. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (not used — PBF listings are a global catalog)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only listings whose PBF name matches this exactly (optional)"},
	{Name: "name_contains", Type: core.ConnectionTypeString, Label: "Name Contains", Placeholder: "Only listings whose PBF name contains this text (optional)"},
	{Name: "pbf_listing_id", Type: core.ConnectionTypeString, Label: "PBF Listing OCID", Placeholder: "ocid1.fnpbflisting.oc1..aaaa… — return only this listing (optional)"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page, 1–50 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pbf_listings", Type: core.ConnectionTypeObject, Label: "PBF Listings"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	req := functions.ListPbfListingsRequest{}
	if name := fn.OptionalString("name", inputs); name != "" {
		req.Name = &name
	}
	if nameContains := fn.OptionalString("name_contains", inputs); nameContains != "" {
		req.NameContains = &nameContains
	}
	if id := fn.OptionalString("pbf_listing_id", inputs); id != "" {
		req.PbfListingId = &id
	}
	if v, ok, err := fn.OptionalInt64("limit", inputs); err != nil {
		return fn.ErrorResult(err.Error()), nil
	} else if ok {
		n := int(v)
		req.Limit = &n
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= fn.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListPbfListings(fn.Context(), req)
		if err != nil {
			return fn.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			item := &resp.Items[i]
			out = append(out, map[string]interface{}{
				"id":              fn.Str(item.Id),
				"name":            fn.Str(item.Name),
				"description":     fn.Str(item.Description),
				"lifecycle_state": string(item.LifecycleState),
				"time_created":    fn.FormatTime(item.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return fn.Result(fmt.Sprintf("Found %d PBF listing(s)", len(out)), map[string]interface{}{
		"pbf_listings": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
