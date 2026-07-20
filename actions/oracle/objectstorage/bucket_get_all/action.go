// Package oracle_objectstorage_bucket_get_all lists the Object Storage buckets in
// an OCI compartment. The tenancy namespace is resolved automatically.
package oracle_objectstorage_bucket_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	os "flomation.app/automate/executor/actions/oracle/objectstorage"

	ocios "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Object Storage: List Buckets"
	Description  = "List the Object Storage buckets in an Oracle Cloud compartment, with their namespace and creation time. The tenancy namespace is resolved automatically."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+list"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "buckets", Type: core.ConnectionTypeObject, Label: "Buckets"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// maxPages bounds the walk so a compartment with a huge number of buckets (or a
// pagination anomaly) can't turn one list into an unbounded request — matching
// the object/PAR list actions.
const maxPages = 25

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := os.GetAuth(inputs)
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return os.ErrorResult(err.Error()), nil
	}
	client, err := auth.Client()
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}
	ctx := os.Context()
	ns, err := auth.Namespace(ctx, client)
	if err != nil {
		return os.ErrorResult(auth.OCIError(err)), nil
	}

	var buckets []map[string]interface{}
	req := ocios.ListBucketsRequest{NamespaceName: &ns, CompartmentId: &compartment}
	truncated := false
	for page := 0; page < maxPages; page++ {
		resp, err := client.ListBuckets(ctx, req)
		if err != nil {
			return os.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			b := &resp.Items[i]
			m := map[string]interface{}{
				"name":           os.Str(b.Name),
				"namespace":      os.Str(b.Namespace),
				"compartment_id": os.Str(b.CompartmentId),
			}
			if b.TimeCreated != nil {
				m["time_created"] = os.FormatTime(b.TimeCreated)
			}
			buckets = append(buckets, m)
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
		if page == maxPages-1 {
			truncated = true
		}
	}

	summary := fmt.Sprintf("Found %d bucket(s) in namespace %s", len(buckets), ns)
	if truncated {
		summary = fmt.Sprintf("Found at least %d bucket(s) in namespace %s (list truncated at %d pages — more available)", len(buckets), ns, maxPages)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"buckets":     buckets,
		"namespace":   ns,
		"count":       len(buckets),
		"truncated":   truncated,
		"success":     true,
	}, nil
}
