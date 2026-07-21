// Package aws_vpc_create_ipam creates an AWS IPAM (IP Address Manager).
package aws_vpc_create_ipam

import (
	"context"
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
	Name         = "AWS IPAM Create IPAM"
	Description  = "Create an AWS IPAM (IP Address Manager) with operating Regions and tier."
	Website      = "https://www.flomation.co"
	Icon         = "table+plus"
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
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "operating_regions", Type: core.ConnectionTypeString, Label: "Operating Regions", Placeholder: "eu-west-2,us-east-1", Required: true},
	{Name: "tier", Type: core.ConnectionTypeString, Label: "Tier", Options: []core.ConnectionOption{
		{Name: "Free", Value: "free"},
		{Name: "Advanced", Value: "advanced"},
	}},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ipam", Type: core.ConnectionTypeObject, Label: "IPAM"},
	{Name: "ipam_id", Type: core.ConnectionTypeString, Label: "IPAM ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	regions := awscommon.InputStrings("operating_regions", inputs)
	if len(regions) == 0 {
		return nil, fmt.Errorf("operating_regions is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateIpamInput{}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	for _, r := range regions {
		in.OperatingRegions = append(in.OperatingRegions, ec2types.AddIpamOperatingRegion{RegionName: aws.String(r)})
	}
	if t := strings.TrimSpace(awscommon.InputString("tier", inputs)); t != "" {
		in.Tier = ec2types.IpamTier(t)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeIpam,
			Tags:         tags,
		}}
	}

	out, err := client.CreateIpam(ctx, in)
	if err != nil {
		return nil, err
	}

	ipam := map[string]interface{}{}
	id := ""
	if out.Ipam != nil {
		id = aws.ToString(out.Ipam.IpamId)
		ipam = map[string]interface{}{
			"ipam_id":     id,
			"ipam_arn":    aws.ToString(out.Ipam.IpamArn),
			"ipam_region": aws.ToString(out.Ipam.IpamRegion),
			"state":       string(out.Ipam.State),
			"tier":        string(out.Ipam.Tier),
			"owner_id":    aws.ToString(out.Ipam.OwnerId),
			"description": aws.ToString(out.Ipam.Description),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created IPAM %s", id),
		"ipam":        ipam,
		"ipam_id":     id,
	}, nil
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
