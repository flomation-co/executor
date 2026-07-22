// Package oracle_datacatalog_connection_create creates a connection to a data asset — the set of
// type-specific credentials and endpoints Data Catalog uses to reach and harvest a physical store.
// A connection is a child of its data asset: it is identified by an immutable string Key, not an
// OCID, and is created under a catalog + data-asset key.
package oracle_datacatalog_connection_create

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dc "flomation.app/automate/executor/actions/oracle/datacatalog"

	"github.com/oracle/oci-go-sdk/v65/datacatalog"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Catalog: Create Connection"
	Description  = "Create a connection to a data asset. Supply the connection type key plus its type-specific properties (host, username, …) and any encrypted properties (password, …); returns the connection with its immutable Key."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+book"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "catalog_ocid", Type: core.ConnectionTypeString, Label: "Catalog OCID", Placeholder: "ocid1.datacatalog.oc1..aaaa… (the catalog the data asset lives in)", Required: true},
	{Name: "data_asset_key", Type: core.ConnectionTypeString, Label: "Data Asset Key", Placeholder: "The parent data asset's key (not an OCID)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the connection", Required: true},
	{Name: "type_key", Type: core.ConnectionTypeString, Label: "Connection Type Key", Placeholder: "The connection type's key — list the data asset's connection types to find it", Required: true},
	{Name: "properties", Type: core.ConnectionTypeText, Label: "Properties (JSON)", Placeholder: "Type-specific properties keyed by category, e.g. {\"default\":{\"host\":\"db1\",\"username\":\"user1\"}} (optional)"},
	{Name: "enc_properties", Type: core.ConnectionTypeText, Label: "Encrypted Properties (JSON)", Placeholder: "Sensitive properties keyed by category, e.g. {\"default\":{\"password\":\"secret\"}} (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "is_default", Type: core.ConnectionTypeBoolean, Label: "Default Connection", Placeholder: "Make this the data asset's default connection (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "connection", Type: core.ConnectionTypeObject, Label: "Connection"},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Connection Key"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// parsePropertyMap decodes a "category → property → value" JSON object into the nested map shape the
// Data Catalog connection properties use. Blank input returns nil so the caller can decide a default.
func parsePropertyMap(name string, inputs []*core.Connection) (map[string]map[string]string, error) {
	raw := strings.TrimSpace(dc.OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var m map[string]map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of category maps, e.g. {\"default\":{\"host\":\"db1\"}}: %s", strings.ReplaceAll(name, "_", " "), err.Error())
	}
	return m, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dc.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	catalogID, err := dc.RequiredString("catalog_ocid", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	dataAssetKey, err := dc.RequiredString("data_asset_key", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	displayName, err := dc.RequiredString("display_name", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	typeKey, err := dc.RequiredString("type_key", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}

	// Properties is a mandatory field on the SDK details — a nil map serialises to null and 400s, so
	// fall back to a non-nil empty map when the operator supplies none.
	props, err := parsePropertyMap("properties", inputs)
	if err != nil {
		return dc.ErrorResult(err.Error()), nil
	}
	if props == nil {
		props = map[string]map[string]string{}
	}

	details := datacatalog.CreateConnectionDetails{
		DisplayName: &displayName,
		TypeKey:     &typeKey,
		Properties:  props,
	}
	if enc, err := parsePropertyMap("enc_properties", inputs); err != nil {
		return dc.ErrorResult(err.Error()), nil
	} else if enc != nil {
		details.EncProperties = enc
	}
	if d := dc.OptionalString("description", inputs); strings.TrimSpace(d) != "" {
		details.Description = &d
	}
	if p := dc.OptionalBoolPtr("is_default", inputs); p != nil {
		details.IsDefault = p
	}

	resp, err := client.CreateConnection(dc.Context(), datacatalog.CreateConnectionRequest{
		CatalogId:               &catalogID,
		DataAssetKey:            &dataAssetKey,
		CreateConnectionDetails: details,
	})
	if err != nil {
		return dc.ErrorResult(auth.OCIError(err)), nil
	}

	conn := dc.SummariseConnection(&resp.Connection)
	return dc.Result(fmt.Sprintf("Created connection %q (%s)", conn["display_name"], conn["lifecycle_state"]), map[string]interface{}{
		"connection": conn,
		"key":        conn["key"],
	}), nil
}
