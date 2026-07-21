// Package aws_vpc_create_managed_prefix_list creates a customer-managed prefix list.
package aws_vpc_create_managed_prefix_list

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
	Name         = "AWS VPC Create Managed Prefix List"
	Description  = "Create a customer-managed prefix list of CIDR entries."
	Website      = "https://www.flomation.co"
	Icon         = "list+plus"
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
	{Name: "prefix_list_name", Type: core.ConnectionTypeString, Label: "Prefix List Name", Placeholder: "office-networks", Required: true},
	{Name: "max_entries", Type: core.ConnectionTypeInteger, Label: "Maximum Entries", Placeholder: "The maximum number of CIDR entries the list can hold", Required: true},
	{Name: "address_family", Type: core.ConnectionTypeString, Label: "Address Family", Required: true, Options: []core.ConnectionOption{
		{Name: "IPv4", Value: "IPv4"},
		{Name: "IPv6", Value: "IPv6"},
	}},
	{Name: "entries", Type: core.ConnectionTypeText, Label: "Entries (optional)", Placeholder: `JSON array e.g. [{"cidr":"10.0.0.0/16","description":"HQ"}]`},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "prefix_list", Type: core.ConnectionTypeObject, Label: "Prefix List"},
	{Name: "prefix_list_id", Type: core.ConnectionTypeString, Label: "Prefix List ID"},
}

type entryInput struct {
	Cidr        string `json:"cidr"`
	Description string `json:"description"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("prefix_list_name", inputs))
	if name == "" {
		return nil, fmt.Errorf("prefix_list_name is required")
	}
	maxEntries, ok := intInput("max_entries", inputs)
	if !ok {
		return nil, fmt.Errorf("max_entries is required")
	}
	family := strings.TrimSpace(awscommon.InputString("address_family", inputs))
	if family == "" {
		return nil, fmt.Errorf("address_family is required")
	}

	entries, err := parseAddEntries("entries", inputs)
	if err != nil {
		return nil, err
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateManagedPrefixListInput{
		PrefixListName: aws.String(name),
		MaxEntries:     aws.Int32(maxEntries),
		AddressFamily:  aws.String(family),
	}
	if len(entries) > 0 {
		in.Entries = entries
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypePrefixList,
			Tags:         tags,
		}}
	}

	out, err := client.CreateManagedPrefixList(ctx, in)
	if err != nil {
		return nil, err
	}

	pl := map[string]interface{}{}
	id := ""
	if out.PrefixList != nil {
		id = aws.ToString(out.PrefixList.PrefixListId)
		pl = map[string]interface{}{
			"prefix_list_id":   id,
			"prefix_list_name": aws.ToString(out.PrefixList.PrefixListName),
			"state":            string(out.PrefixList.State),
			"address_family":   aws.ToString(out.PrefixList.AddressFamily),
			"max_entries":      aws.ToInt32(out.PrefixList.MaxEntries),
			"version":          aws.ToInt64(out.PrefixList.Version),
		}
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Created managed prefix list %s (%s)", id, name),
		"prefix_list":    pl,
		"prefix_list_id": id,
	}, nil
}

// parseAddEntries reads a JSON array of {cidr, description} from a text input.
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

func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
