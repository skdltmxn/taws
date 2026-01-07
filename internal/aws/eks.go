package aws

import (
	"context"
	"fmt"
	"sort"

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
	var lastErr error
	for _, name := range output.Clusters {
		desc, err := c.client.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(name),
		})
		if err != nil {
			lastErr = err
			clusters = append(clusters, domain.EKSCluster{
				Name:   name,
				Status: "Unknown",
			})
			continue
		}

		cluster := desc.Cluster
		clusters = append(clusters, domain.EKSCluster{
			Name:      derefString(cluster.Name),
			Status:    string(cluster.Status),
			Version:   derefString(cluster.Version),
			CreatedAt: derefTime(cluster.CreatedAt),
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	if lastErr != nil && len(clusters) == 0 {
		return nil, fmt.Errorf("failed to describe clusters: %w", lastErr)
	}

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
				nodegroups = append(nodegroups, domain.EKSNodegroup{
					Name:   ngName,
					Status: "Unknown",
				})
				continue
			}

			ng := desc.Nodegroup
			name := derefString(ng.NodegroupName)
			if name == "" {
				name = ngName
			}

			nodegroups = append(nodegroups, domain.EKSNodegroup{
				Name:      name,
				Status:    string(ng.Status),
				Version:   derefString(ng.Version),
				CreatedAt: derefTime(ng.CreatedAt),
			})
		}
	}

	sort.Slice(nodegroups, func(i, j int) bool {
		return nodegroups[i].Name < nodegroups[j].Name
	})

	return nodegroups, nil
}
