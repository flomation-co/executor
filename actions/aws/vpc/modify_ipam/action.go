// Package aws_vpc_modify_ipam modifies an AWS IPAM.
package aws_vpc_modify_ipam

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
	Name         = "AWS IPAM Modify IPAM"
	Description  = "Modify an AWS IPAM's description or operating Regions."
	Website      = "https://www.flomation.co"
	Icon         = "table+pen"
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
	{Name: "ipam_id", Type: core.ConnectionTypeString, Label: "IPAM ID", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "add_operating_regions", Type: core.ConnectionTypeString, Label: "Add Operating Regions (optional)", Placeholder: "us-east-1,eu-west-1"},
	{Name: "remove_operating_regions", Type: core.ConnectionTypeString, Label: "Remove Operating Regions (optional)", Placeholder: "us-east-1"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ipam", Type: core.ConnectionTypeObject, Label: "IPAM"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("ipam_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("ipam_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ModifyIpamInput{IpamId: aws.String(id)}
	changed := false
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
		changed = true
	}
	for _, r := range awscommon.InputStrings("add_operating_regions", inputs) {
		in.AddOperatingRegions = append(in.AddOperatingRegions, ec2types.AddIpamOperatingRegion{RegionName: aws.String(r)})
		changed = true
	}
	for _, r := range awscommon.InputStrings("remove_operating_regions", inputs) {
		in.RemoveOperatingRegions = append(in.RemoveOperatingRegions, ec2types.RemoveIpamOperatingRegion{RegionName: aws.String(r)})
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("provide at least one of description, add_operating_regions, or remove_operating_regions")
	}

	out, err := client.ModifyIpam(ctx, in)
	if err != nil {
		return nil, err
	}

	ipam := map[string]interface{}{}
	if out.Ipam != nil {
		ipam = map[string]interface{}{
			"ipam_id":     aws.ToString(out.Ipam.IpamId),
			"ipam_arn":    aws.ToString(out.Ipam.IpamArn),
			"ipam_region": aws.ToString(out.Ipam.IpamRegion),
			"state":       string(out.Ipam.State),
			"tier":        string(out.Ipam.Tier),
			"description": aws.ToString(out.Ipam.Description),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified IPAM %s", id),
		"ipam":        ipam,
	}, nil
}
