package domain

import "context"

type HostedZone struct {
	ID    string
	Name  string
	Count int64
}

type Route53RecordSet struct {
	ID    string
	Name  string
	Type  string
	TTL   int64
	Value string
}

type Route53Client interface {
	ListHostedZones(ctx context.Context) ([]HostedZone, error)
	ListRecordSets(ctx context.Context, hostedZoneID string) ([]Route53RecordSet, error)
}
