package aws

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/skdltmxn/taws/internal/domain"
)

type VPCClient struct {
	client *ec2.Client
}

func NewVPCClient(cfg aws.Config) *VPCClient {
	return &VPCClient{
		client: ec2.NewFromConfig(cfg),
	}
}

func (c *VPCClient) ListVPCs(ctx context.Context) ([]domain.VPC, error) {
	input := &ec2.DescribeVpcsInput{}
	var vpcs []domain.VPC

	paginator := ec2.NewDescribeVpcsPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe vpcs: %w", err)
		}

		for _, v := range output.Vpcs {
			vpcs = append(vpcs, domain.VPC{
				ID:        derefString(v.VpcId),
				CidrBlock: derefString(v.CidrBlock),
				State:     string(v.State),
				IsDefault: derefBool(v.IsDefault),
				Name:      extractNameTag(v.Tags),
			})
		}
	}

	sort.Slice(vpcs, func(i, j int) bool {
		return vpcs[i].Name < vpcs[j].Name
	})

	return vpcs, nil
}

func (c *VPCClient) ListSubnets(ctx context.Context, vpcID string) ([]domain.Subnet, error) {
	input := &ec2.DescribeSubnetsInput{}
	if vpcID != "" {
		input.Filters = []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		}
	}

	var subnets []domain.Subnet
	paginator := ec2.NewDescribeSubnetsPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe subnets: %w", err)
		}

		for _, s := range output.Subnets {
			subnets = append(subnets, domain.Subnet{
				ID:                  derefString(s.SubnetId),
				VpcID:               derefString(s.VpcId),
				CidrBlock:           derefString(s.CidrBlock),
				AvailabilityZone:    derefString(s.AvailabilityZone),
				AvailableIPs:        derefInt32(s.AvailableIpAddressCount),
				State:               string(s.State),
				Name:                extractNameTag(s.Tags),
				MapPublicIPOnLaunch: derefBool(s.MapPublicIpOnLaunch),
			})
		}
	}

	sort.Slice(subnets, func(i, j int) bool {
		return subnets[i].Name < subnets[j].Name
	})

	return subnets, nil
}
