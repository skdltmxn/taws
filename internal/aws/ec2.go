package aws

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/skdltmxn/taws/internal/domain"
)

type EC2Client struct {
	client *ec2.Client
}

func NewEC2Client(cfg aws.Config) *EC2Client {
	return &EC2Client{
		client: ec2.NewFromConfig(cfg),
	}
}

func (c *EC2Client) ListInstances(ctx context.Context) ([]domain.EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	var instances []domain.EC2Instance

	paginator := ec2.NewDescribeInstancesPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				name := ""
				for _, tag := range instance.Tags {
					if *tag.Key == "Name" {
						name = *tag.Value
						break
					}
				}

				publicIP := ""
				if instance.PublicIpAddress != nil {
					publicIP = *instance.PublicIpAddress
				}

				privateIP := ""
				if instance.PrivateIpAddress != nil {
					privateIP = *instance.PrivateIpAddress
				}

				keyName := ""
				if instance.KeyName != nil {
					keyName = *instance.KeyName
				}

				vpcID := ""
				if instance.VpcId != nil {
					vpcID = *instance.VpcId
				}

				subnetID := ""
				if instance.SubnetId != nil {
					subnetID = *instance.SubnetId
				}

				platform := "Linux"
				if instance.Platform != "" {
					platform = string(instance.Platform)
				}

				az := ""
				if instance.Placement != nil && instance.Placement.AvailabilityZone != nil {
					az = *instance.Placement.AvailabilityZone
				}

				instances = append(instances, domain.EC2Instance{
					ID:               *instance.InstanceId,
					Name:             name,
					Type:             string(instance.InstanceType),
					State:            domain.InstanceStatus(instance.State.Name),
					PublicIP:         publicIP,
					PrivateIP:        privateIP,
					LaunchTime:       *instance.LaunchTime,
					KeyName:          keyName,
					VpcID:            vpcID,
					SubnetID:         subnetID,
					AvailabilityZone: az,
					Architecture:     string(instance.Architecture),
					Platform:         platform,
				})
			}
		}
	}

	// Sort by Name, then ID
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Name == instances[j].Name {
			return instances[i].ID < instances[j].ID
		}
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

func (c *EC2Client) StartInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
	}
	return nil
}

func (c *EC2Client) StopInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}
	return nil
}

func (c *EC2Client) RebootInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.RebootInstances(ctx, &ec2.RebootInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to reboot instance %s: %w", instanceID, err)
	}
	return nil
}

func (c *EC2Client) TerminateInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}
	return nil
}
