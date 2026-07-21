// Package aws_vpc_modify_managed_prefix_list modifies a customer-managed prefix list.
package aws_vpc_modify_managed_prefix_list

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Modify Managed Prefix List"
	Description  = "Rename a managed prefix list, or add/remove CIDR entries."
	Website      = "https://www.flomation.co"
	Icon         = "list+pen"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "prefix_list_id", Type: core.ConnectionTypeString, Label: "Prefix List ID", Placeholder: "pl-0abc...", Required: true},
	{Name: "prefix_list_name", Type: core.ConnectionTypeString, Label: "New Name (optional)", Placeholder: "Leave blank to keep current name"},
	{Name: "add_entries", Type: core.ConnectionTypeText, Label: "Add Entries (optional)", Placeholder: `JSON array e.g. [{"cidr":"10.0.0.0/16","description":"HQ"}]`},
	{Name: "remove_entries", Type: core.ConnectionTypeText, Label: "Remove Entries (optional)", Placeholder: `JSON array e.g. [{"cidr":"10.0.0.0/16"}]`},
	{Name: "current_version", Type: core.ConnectionTypeInteger, Label: "Current Version (optional)", Placeholder: "Required when adding or removing entries"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "prefix_list", Type: core.ConnectionTypeObject, Label: "Prefix List"},
}

type entryInput struct {
	Cidr        string `json:"cidr"`
	Description string `json:"description"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	plID := strings.TrimSpace(awscommon.InputString("prefix_list_id", inputs))
	if plID == "" {
		return nil, fmt.Errorf("prefix_list_id is required")
	}
	newName := strings.TrimSpace(awscommon.InputString("prefix_list_name", inputs))

	addEntries, err := parseAddEntries("add_entries", inputs)
	if err != nil {
		return nil, err
	}
	removeEntries, err := parseRemoveEntries("remove_entries", inputs)
	if err != nil {
		return nil, err
	}

	if newName == "" && len(addEntries) == 0 && len(removeEntries) == 0 {
		return nil, fmt.Errorf("provide a new name, entries to add, or entries to remove")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ModifyManagedPrefixListInput{PrefixListId: aws.String(plID)}
	if newName != "" {
		in.PrefixListName = aws.String(newName)
	}
	if len(addEntries) > 0 {
		in.AddEntries = addEntries
	}
	if len(removeEntries) > 0 {
		in.RemoveEntries = removeEntries
	}
	if v, ok := intInput("current_version", inputs); ok {
		in.CurrentVersion = aws.Int64(int64(v))
	}

	out, err := client.ModifyManagedPrefixList(ctx, in)
	if err != nil {
		return nil, err
	}

	pl := map[string]interface{}{}
	if out.PrefixList != nil {
		pl = map[string]interface{}{
			"prefix_list_id":   aws.ToString(out.PrefixList.PrefixListId),
			"prefix_list_name": aws.ToString(out.PrefixList.PrefixListName),
			"state":            string(out.PrefixList.State),
			"version":          aws.ToInt64(out.PrefixList.Version),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified managed prefix list %s (+%d, -%d entries)", plID, len(addEntries), len(removeEntries)),
		"prefix_list": pl,
	}, nil
}

func parseAddEntries(name string, inputs []*core.Connection) ([]ec2types.AddPrefixListEntry, error) {
	raw := strings.TrimSpace(awscommon.InputString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var parsed []entryInput
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of {cidr, description}: %w", name, err)
	}
	var out []ec2types.AddPrefixListEntry
	for _, e := range parsed {
		cidr := strings.TrimSpace(e.Cidr)
		if cidr == "" {
			continue
		}
		entry := ec2types.AddPrefixListEntry{Cidr: aws.String(cidr)}
		if d := strings.TrimSpace(e.Description); d != "" {
			entry.Description = aws.String(d)
		}
		out = append(out, entry)
	}
	return out, nil
}

func parseRemoveEntries(name string, inputs []*core.Connection) ([]ec2types.RemovePrefixListEntry, error) {
	raw := strings.TrimSpace(awscommon.InputString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var parsed []entryInput
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of {cidr}: %w", name, err)
	}
	var out []ec2types.RemovePrefixListEntry
	for _, e := range parsed {
		cidr := strings.TrimSpace(e.Cidr)
		if cidr == "" {
			continue
		}
		out = append(out, ec2types.RemovePrefixListEntry{Cidr: aws.String(cidr)})
	}
	return out, nil
}

func intInput(name string, inputs []*core.Connection) (int32, bool) {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return 0, false
	}
	n := c.Number()
	if n == nil {
		return 0, false
	}
	return int32(*n), true
}
