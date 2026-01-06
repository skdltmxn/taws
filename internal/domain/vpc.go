package domain

import "context"

type VPC struct {
	ID        string
	CidrBlock string
	State     string
	IsDefault bool
	Name      string
}

type Subnet struct {
	ID                  string
	VpcID               string
	CidrBlock           string
	AvailabilityZone    string
	AvailableIPs        int32
	State               string
	Name                string
	MapPublicIPOnLaunch bool
}

type VPCClient interface {
	ListVPCs(ctx context.Context) ([]VPC, error)
	ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error)
}
