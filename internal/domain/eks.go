package domain

import (
	"context"
	"time"
)

type EKSCluster struct {
	Name      string
	Status    string
	Version   string
	CreatedAt time.Time
}

type EKSNodegroup struct {
	Name      string
	Status    string
	Version   string
	CreatedAt time.Time
}

type EKSClient interface {
	ListClusters(ctx context.Context) ([]EKSCluster, error)
	ListNodegroups(ctx context.Context, clusterName string) ([]EKSNodegroup, error)
}
