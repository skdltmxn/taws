package domain

import (
	"context"
	"time"
)

type InstanceStatus string

const (
	InstanceStatusRunning      InstanceStatus = "running"
	InstanceStatusStopped      InstanceStatus = "stopped"
	InstanceStatusTerminated   InstanceStatus = "terminated"
	InstanceStatusPending      InstanceStatus = "pending"
	InstanceStatusStopping     InstanceStatus = "stopping"
	InstanceStatusShuttingDown InstanceStatus = "shutting-down"
)

type EC2Instance struct {
	ID               string
	Name             string
	Type             string
	State            InstanceStatus
	PublicIP         string
	PrivateIP        string
	LaunchTime       time.Time
	KeyName          string
	VpcID            string
	SubnetID         string
	AvailabilityZone string
	Architecture     string
	Platform         string
}

type EC2Client interface {
	ListInstances(ctx context.Context) ([]EC2Instance, error)
	StartInstance(ctx context.Context, instanceID string) error
	StopInstance(ctx context.Context, instanceID string) error
	RebootInstance(ctx context.Context, instanceID string) error
	TerminateInstance(ctx context.Context, instanceID string) error
}
