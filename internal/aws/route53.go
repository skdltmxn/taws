package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/skdltmxn/taws/internal/domain"
)

type Route53Client struct {
	client *route53.Client
}

func NewRoute53Client(cfg aws.Config) *Route53Client {
	return &Route53Client{
		client: route53.NewFromConfig(cfg),
	}
}

func (c *Route53Client) ListHostedZones(ctx context.Context) ([]domain.HostedZone, error) {
	var zones []domain.HostedZone

	paginator := route53.NewListHostedZonesPaginator(c.client, &route53.ListHostedZonesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list hosted zones: %w", err)
		}
		for _, z := range output.HostedZones {
			id := ""
			if z.Id != nil {
				id = *z.Id
			}
			name := ""
			if z.Name != nil {
				name = *z.Name
			}
			var count int64
			if z.ResourceRecordSetCount != nil {
				count = *z.ResourceRecordSetCount
			}
			zones = append(zones, domain.HostedZone{
				ID:    id,
				Name:  name,
				Count: count,
			})
		}
	}

	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Name < zones[j].Name
	})

	return zones, nil
}

func (c *Route53Client) ListRecordSets(ctx context.Context, hostedZoneID string) ([]domain.Route53RecordSet, error) {
	if hostedZoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}

	var records []domain.Route53RecordSet
	paginator := route53.NewListResourceRecordSetsPaginator(c.client, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list record sets: %w", err)
		}

		for _, r := range output.ResourceRecordSets {
			name := ""
			if r.Name != nil {
				name = *r.Name
			}

			typ := ""
			if r.Type != "" {
				typ = string(r.Type)
			}

			var ttl int64
			if r.TTL != nil {
				ttl = *r.TTL
			}

			value := formatRecordSetValue(r)
			id := name + "|" + typ
			if r.SetIdentifier != nil && *r.SetIdentifier != "" {
				id = id + "|" + *r.SetIdentifier
			}

			records = append(records, domain.Route53RecordSet{
				ID:    id,
				Name:  name,
				Type:  typ,
				TTL:   ttl,
				Value: value,
			})
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Name == records[j].Name {
			return records[i].Type < records[j].Type
		}
		return records[i].Name < records[j].Name
	})

	return records, nil
}

func formatRecordSetValue(r route53types.ResourceRecordSet) string {
	if r.AliasTarget != nil && r.AliasTarget.DNSName != nil && *r.AliasTarget.DNSName != "" {
		return "ALIAS " + *r.AliasTarget.DNSName
	}

	if len(r.ResourceRecords) > 0 {
		values := make([]string, 0, len(r.ResourceRecords))
		for _, rr := range r.ResourceRecords {
			if rr.Value == nil || *rr.Value == "" {
				continue
			}
			values = append(values, *rr.Value)
		}
		if len(values) == 0 {
			return "-"
		}
		if len(values) <= 2 {
			return strings.Join(values, ", ")
		}
		return strings.Join(values[:2], ", ") + fmt.Sprintf(" (+%d)", len(values)-2)
	}

	if r.TrafficPolicyInstanceId != nil && *r.TrafficPolicyInstanceId != "" {
		return "TrafficPolicy " + *r.TrafficPolicyInstanceId
	}

	return "-"
}
