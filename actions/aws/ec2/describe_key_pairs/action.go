// Package aws_ec2_describe_key_pairs lists EC2 key pairs.
package aws_ec2_describe_key_pairs

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Describe Key Pairs"
	Description  = "List EC2 key pairs with their name, id, type and fingerprint."
	Website      = "https://www.flomation.co"
	Icon         = "key"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)"},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/MyRole"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_pairs", Type: core.ConnectionTypeObject, Label: "Key Pairs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{})
	if err != nil {
		return nil, err
	}

	var pairs []map[string]interface{}
	for _, k := range out.KeyPairs {
		pairs = append(pairs, map[string]interface{}{
			"key_name":    aws.ToString(k.KeyName),
			"key_pair_id": aws.ToString(k.KeyPairId),
			"type":        string(k.KeyType),
			"fingerprint": aws.ToString(k.KeyFingerprint),
		})
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d key pair(s)", len(pairs)),
		"key_pairs":   pairs,
		"count":       len(pairs),
	}, nil
}
