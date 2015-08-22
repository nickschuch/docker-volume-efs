package main

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Helper function to get a subnet which an EC2 instance belong to.
func GetSubnet(e *ec2.EC2, i string) (string, error) {
	describeParams := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(i),
		},
	}
	describeResp, err := e.DescribeInstances(describeParams)
	if err != nil {
		return "", err
	}

	// Ensure we got a result from this query.
	if len(describeResp.Reservations) <= 0 {
		return "", errors.New("Cannot find this host by AWS EC2 DescribeInstances API")
	}
	if len(describeResp.Reservations[0].Instances) <= 0 {
		return "", errors.New("Cannot find this host by AWS EC2 DescribeInstances API")
	}

	return *describeResp.Reservations[0].Instances[0].SubnetId, nil
}
