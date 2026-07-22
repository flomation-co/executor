// Package oracle_filestorage_outbound_connector_update renames an Oracle Cloud File
// Storage outbound connector and/or replaces its free-form tags.
package oracle_filestorage_outbound_connector_update

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
	Name         = "OCI File Storage: Update Outbound Connector"
	Description  = "Rename an Oracle Cloud File Storage outbound connector and/or set its free-form tags. Only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the outbound-connector picker)"},
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1 (scopes the outbound-connector picker)"},
	{Name: "outbound_connector_ocid", Type: core.ConnectionTypeString, Label: "Outbound Connector OCID", Placeholder: "ocid1.outboundconnector.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name for the outbound connector (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Free-form Tags", Placeholder: "JSON object, e.g. {\"env\":\"prod\"} — replaces the existing free-form tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "outbound_connector", Type: core.ConnectionTypeObject, Label: "Outbound Connector"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Outbound Connector OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "outbound_connector_ocid")
	if errResult != nil {
		return errResult, nil
	}

	var details filestorage.UpdateOutboundConnectorDetails
	if name := strings.TrimSpace(fss.OptionalString("display_name", inputs)); name != "" {
		details.DisplayName = &name
	}
	if tags, err := fss.FreeformTags("tags", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateOutboundConnector(fss.Context(), filestorage.UpdateOutboundConnectorRequest{
		OutboundConnectorId:            &id,
		UpdateOutboundConnectorDetails: details,
	})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	oc := resp.OutboundConnector
	connector := map[string]interface{}{
		"id":                  fss.Str(oc.GetId()),
		"display_name":        fss.Str(oc.GetDisplayName()),
		"compartment_id":      fss.Str(oc.GetCompartmentId()),
		"availability_domain": fss.Str(oc.GetAvailabilityDomain()),
		"lifecycle_state":     string(oc.GetLifecycleState()),
		"freeform_tags":       oc.GetFreeformTags(),
		"time_created":        fss.FormatTime(oc.GetTimeCreated()),
	}

	return fss.Result(fmt.Sprintf("Outbound connector %q is %s", connector["display_name"], connector["lifecycle_state"]), map[string]interface{}{
		"outbound_connector": connector,
		"id":                 connector["id"],
		"lifecycle_state":    connector["lifecycle_state"],
	}), nil
}
