package rds

import (
	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// BuildServerlessV2 assembles an Aurora Serverless v2 scaling configuration from
// the standard `serverless_v2_min_capacity` / `serverless_v2_max_capacity`
// inputs (ACU, e.g. 0.5–16). Returns nil when neither is set, so a provisioned
// (non-Serverless) cluster isn't given a spurious scaling block. Shared by the
// create and modify cluster actions.
func BuildServerlessV2(inputs []*core.Connection) *rdstypes.ServerlessV2ScalingConfiguration {
	min, hasMin := awscommon.InputFloat("serverless_v2_min_capacity", inputs)
	max, hasMax := awscommon.InputFloat("serverless_v2_max_capacity", inputs)
	if !hasMin && !hasMax {
		return nil
	}
	cfg := &rdstypes.ServerlessV2ScalingConfiguration{}
	if hasMin {
		cfg.MinCapacity = awssdk.Float64(min)
	}
	if hasMax {
		cfg.MaxCapacity = awssdk.Float64(max)
	}
	return cfg
}
