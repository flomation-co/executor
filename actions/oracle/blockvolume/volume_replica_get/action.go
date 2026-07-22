// Package oracle_blockvolume_volume_replica_get reads one cross-region block-volume replica by OCID.
package oracle_blockvolume_volume_replica_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	bv "flomation.app/automate/executor/actions/oracle/blockvolume"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Block Volumes: Get Block Volume Replica"
	Description  = "Fetch a single Oracle Cloud cross-region block-volume replica by OCID — its source volume, availability domain, lifecycle state and last-synced time."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+copy"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the replica picker)"},
	{Name: "block_volume_replica_ocid", Type: core.ConnectionTypeString, Label: "Block Volume Replica OCID", Placeholder: "ocid1.blockvolumereplica.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "replica", Type: core.ConnectionTypeObject, Label: "Block Volume Replica"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Replica OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := bv.VolumeResourceClient(inputs, "block_volume_replica_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetBlockVolumeReplica(bv.Context(), ocicore.GetBlockVolumeReplicaRequest{BlockVolumeReplicaId: &id})
	if err != nil {
		return bv.ErrorResult(auth.OCIError(err)), nil
	}
	replica := bv.SummariseBlockVolumeReplica(&resp.BlockVolumeReplica)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Block volume replica %q is %s", replica["display_name"], replica["lifecycle_state"]),
		"replica":         replica,
		"id":              replica["id"],
		"lifecycle_state": replica["lifecycle_state"],
		"success":         true,
	}, nil
}
