// Package aws_vpc_create_network_insights_path creates a Reachability Analyzer path.
package aws_vpc_create_network_insights_path

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Create Network Insights Path"
	Description  = "Create a Reachability Analyzer path between a source and destination resource."
	Website      = "https://www.flomation.co"
	Icon         = "route+plus"
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
	{Name: "source", Type: core.ConnectionTypeString, Label: "Source", Placeholder: "Resource ID or ARN (e.g. igw-0abc... or an instance ID)", Required: true},
	{Name: "destination", Type: core.ConnectionTypeString, Label: "Destination (optional)", Placeholder: "Resource ID or ARN"},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: []core.ConnectionOption{
		{Name: "TCP", Value: "tcp"},
		{Name: "UDP", Value: "udp"},
	}},
	{Name: "destination_port", Type: core.ConnectionTypeInteger, Label: "Destination Port (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "path", Type: core.ConnectionTypeObject, Label: "Network Insights Path"},
	{Name: "network_insights_path_id", Type: core.ConnectionTypeString, Label: "Path ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	source := strings.TrimSpace(awscommon.InputString("source", inputs))
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	protocol := strings.TrimSpace(awscommon.InputString("protocol", inputs))
	if protocol == "" {
		return nil, fmt.Errorf("protocol is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateNetworkInsightsPathInput{
		ClientToken: aws.String(uuid.NewString()),
		Source:      aws.String(source),
		Protocol:    ec2types.Protocol(protocol),
	}
	if d := strings.TrimSpace(awscommon.InputString("destination", inputs)); d != "" {
		in.Destination = aws.String(d)
	}
	if p, ok := awscommon.InputInt("destination_port", inputs); ok {
		in.DestinationPort = aws.Int32(int32(p))
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeNetworkInsightsPath,
			Tags:         tags,
		}}
	}

	out, err := client.CreateNetworkInsightsPath(ctx, in)
	if err != nil {
		return nil, err
	}

	path := map[string]interface{}{}
	id := ""
	if out.NetworkInsightsPath != nil {
		p := out.NetworkInsightsPath
		id = aws.ToString(p.NetworkInsightsPathId)
		path = map[string]interface{}{
			"network_insights_path_id":  id,
			"network_insights_path_arn": aws.ToString(p.NetworkInsightsPathArn),
			"protocol":                  string(p.Protocol),
			"source":                    aws.ToString(p.Source),
			"destination":               aws.ToString(p.Destination),
			"destination_port":          aws.ToInt32(p.DestinationPort),
		}
	}

	return map[string]interface{}{
		"tool_result":              fmt.Sprintf("Created network insights path %s", id),
		"path":                     path,
		"network_insights_path_id": id,
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
