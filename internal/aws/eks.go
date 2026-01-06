package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/skdltmxn/taws/internal/domain"
)

type EKSClient struct {
	client *eks.Client
}

func NewEKSClient(cfg aws.Config) *EKSClient {
	return &EKSClient{
		client: eks.NewFromConfig(cfg),
	}
}

func (c *EKSClient) ListClusters(ctx context.Context) ([]domain.EKSCluster, error) {
	output, err := c.client.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	var clusters []domain.EKSCluster
	for _, name := range output.Clusters {
		// Describe cluster to get details
		desc, err := c.client.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(name),
		})
		if err != nil {
			// Log error but continue? Or return partial?
			// For now, just skip or return minimal info if describe fails
			// But skipping is safer
			continue
		}

		cluster := desc.Cluster
		clusters = append(clusters, domain.EKSCluster{
			Name:      *cluster.Name,
			Status:    string(cluster.Status),
			Version:   *cluster.Version,
			CreatedAt: *cluster.CreatedAt,
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	return clusters, nil
}

func (c *EKSClient) ListNodegroups(ctx context.Context, clusterName string) ([]domain.EKSNodegroup, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is required")
	}

	var nodegroups []domain.EKSNodegroup
	paginator := eks.NewListNodegroupsPaginator(c.client, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list nodegroups: %w", err)
		}

		for _, ngName := range output.Nodegroups {
			desc, err := c.client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				continue
			}

			ng := desc.Nodegroup
			createdAt := time.Time{}
			if ng.CreatedAt != nil {
				createdAt = *ng.CreatedAt
			}

			version := ""
			if ng.Version != nil {
				version = *ng.Version
			}

			status := ""
			if ng.Status != "" {
				status = string(ng.Status)
			}

			name := ""
			if ng.NodegroupName != nil {
				name = *ng.NodegroupName
			} else {
				name = ngName
			}

			nodegroups = append(nodegroups, domain.EKSNodegroup{
				Name:      name,
				Status:    status,
				Version:   version,
				CreatedAt: createdAt,
			})
		}
	}

	sort.Slice(nodegroups, func(i, j int) bool {
		return nodegroups[i].Name < nodegroups[j].Name
	})

	return nodegroups, nil
}
