package aws_ec2_describe

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Describe"
	Description  = "Describe EC2 instances, returning details like state, type, IPs, and tags"
	Website      = "https://www.flomation.co"
	Icon         = "server"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	return nil, nil
}
