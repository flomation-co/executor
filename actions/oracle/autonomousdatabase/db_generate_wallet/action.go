// Package oracle_autonomousdatabase_db_generate_wallet downloads the client
// credentials (instance) wallet for an Oracle Cloud Autonomous Database — the
// password-protected zip a client needs to connect. The zip is returned base64-
// encoded so it survives round-tripping through a flow.
package oracle_autonomousdatabase_db_generate_wallet

import (
	"encoding/base64"
	"fmt"
	"io"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Generate Wallet"
	Description  = "Download the client credentials (instance) wallet for an Oracle Cloud Autonomous Database — the password-protected zip a client needs to connect. Returned base64-encoded. The password protects the zip and is required."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

// maxWalletBytes bounds the read defensively; ADB wallet zips are only a few KB.
const maxWalletBytes = 10 << 20 // 10 MiB

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "autonomous_database_id", Type: core.ConnectionTypeString, Label: "Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa…", Required: true},
	{Name: "wallet_password", Type: core.ConnectionTypeSecret, Label: "Wallet Password", Placeholder: "Password to protect the wallet zip (min 8 chars)", Required: true},
	{Name: "generate_type", Type: core.ConnectionTypeString, Label: "Wallet Type", Placeholder: "ALL = all connection strings (default); SINGLE = one", Options: []core.ConnectionOption{
		{Name: "All connection strings (default)", Value: "ALL"},
		{Name: "Single connection string", Value: "SINGLE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "wallet_base64", Type: core.ConnectionTypeString, Label: "Wallet (base64 zip)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := adb.PerDatabaseClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	password, err := adb.RequiredString("wallet_password", inputs)
	if err != nil {
		return adb.ErrorResult("wallet password is required (it protects the downloaded zip)"), nil
	}
	details := database.GenerateAutonomousDatabaseWalletDetails{Password: &password}
	if v := adb.OptionalString("generate_type", inputs); v != "" {
		details.GenerateType = database.GenerateAutonomousDatabaseWalletDetailsGenerateTypeEnum(v)
	}
	resp, err := client.GenerateAutonomousDatabaseWallet(adb.Context(), database.GenerateAutonomousDatabaseWalletRequest{
		AutonomousDatabaseId:                    &id,
		GenerateAutonomousDatabaseWalletDetails: details,
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	var data []byte
	if resp.Content != nil {
		defer func() { _ = resp.Content.Close() }()
		data, err = io.ReadAll(io.LimitReader(resp.Content, maxWalletBytes+1))
		if err != nil {
			return adb.ErrorResult(auth.OCIError(err)), nil
		}
		if int64(len(data)) > maxWalletBytes {
			return adb.ErrorResult("wallet download exceeded the expected size — aborting"), nil
		}
	}
	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Generated wallet (%d bytes) for the Autonomous Database", len(data)),
		"wallet_base64": base64.StdEncoding.EncodeToString(data),
		"size_bytes":    len(data),
		"success":       true,
	}, nil
}
