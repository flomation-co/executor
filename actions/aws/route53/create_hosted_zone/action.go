// Package aws_route53_create_hosted_zone creates a Route 53 hosted zone.
package aws_route53_create_hosted_zone

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Create Hosted Zone"
	Description  = "Create a public or private Route 53 hosted zone for a domain."
	Website      = "https://www.flomation.co"
	Icon         = "globe+plus"
	Date         = "22/07/2026"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Domain Name", Placeholder: "example.com", Required: true},
	{Name: "caller_reference", Type: core.ConnectionTypeString, Label: "Caller Reference", Placeholder: "Optional — a unique idempotency token"},
	{Name: "comment", Type: core.ConnectionTypeString, Label: "Comment", Placeholder: "Optional"},
	{Name: "private_zone", Type: core.ConnectionTypeBoolean, Label: "Private Zone"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID (private zone)", Placeholder: "vpc-0abc123"},
	{Name: "vpc_region", Type: core.ConnectionTypeString, Label: "VPC Region (private zone)", Placeholder: "eu-west-2"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "name_servers", Type: core.ConnectionTypeString, Label: "Name Servers (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	callerRef := awscommon.InputString("caller_reference", inputs)
	if callerRef == "" {
		// CallerReference must be non-empty and unique per zone; derive a stable
		// value from the name so a retried request stays idempotent.
		callerRef = "flomation-" + name
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	in := &route53.CreateHostedZoneInput{
		Name:            aws.String(name),
		CallerReference: aws.String(callerRef),
	}

	private := awscommon.InputBool("private_zone", inputs)
	comment := awscommon.InputString("comment", inputs)
	if comment != "" || private {
		hzc := &r53types.HostedZoneConfig{PrivateZone: private}
		if comment != "" {
			hzc.Comment = aws.String(comment)
		}
		in.HostedZoneConfig = hzc
	}

	if vpcID := awscommon.InputString("vpc_id", inputs); vpcID != "" {
		vpc := &r53types.VPC{VPCId: aws.String(vpcID)}
		if vpcRegion := awscommon.InputString("vpc_region", inputs); vpcRegion != "" {
			vpc.VPCRegion = r53types.VPCRegion(vpcRegion)
		}
		in.VPC = vpc
	}

	out, err := client.CreateHostedZone(ctx, in)
	if err != nil {
		return nil, err
	}

	zoneID := aws.ToString(out.HostedZone.Id)
	zoneName := aws.ToString(out.HostedZone.Name)

	var nameServers []string
	if out.DelegationSet != nil {
		nameServers = out.DelegationSet.NameServers
	}
	nsJSON, _ := json.Marshal(nameServers)

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Created hosted zone %s for %s", zoneID, zoneName),
		"hosted_zone_id": zoneID,
		"name":           zoneName,
		"name_servers":   string(nsJSON),
	}, nil
}
