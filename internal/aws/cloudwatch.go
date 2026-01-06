package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/skdltmxn/taws/internal/domain"
)

type CloudWatchClient struct {
	client *cloudwatchlogs.Client
}

func NewCloudWatchClient(cfg aws.Config) *CloudWatchClient {
	return &CloudWatchClient{
		client: cloudwatchlogs.NewFromConfig(cfg),
	}
}

func (c *CloudWatchClient) ListLogGroups(ctx context.Context) ([]domain.CloudWatchLogGroup, error) {
	var groups []domain.CloudWatchLogGroup

	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(c.client, &cloudwatchlogs.DescribeLogGroupsInput{
		Limit: aws.Int32(50),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe log groups: %w", err)
		}
		for _, g := range out.LogGroups {
			groups = append(groups, domain.CloudWatchLogGroup{
				Name:          aws.ToString(g.LogGroupName),
				RetentionDays: aws.ToInt32(g.RetentionInDays),
				StoredBytes:   aws.ToInt64(g.StoredBytes),
			})
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups, nil
}

func (c *CloudWatchClient) FilterLogEvents(ctx context.Context, logGroupName string, startTime time.Time) ([]domain.CloudWatchLogEvent, error) {
	if logGroupName == "" {
		return nil, fmt.Errorf("log group name is required")
	}

	out, err := c.client.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(logGroupName),
		StartTime:    aws.Int64(startTime.UnixMilli()),
		Interleaved:  aws.Bool(true),
		Limit:        aws.Int32(100),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to filter log events: %w", err)
	}

	events := make([]domain.CloudWatchLogEvent, 0, len(out.Events))
	for _, e := range out.Events {
		ts := time.Time{}
		if e.Timestamp != nil {
			ts = time.UnixMilli(*e.Timestamp)
		}
		events = append(events, domain.CloudWatchLogEvent{
			EventID:       aws.ToString(e.EventId),
			Timestamp:     ts,
			Message:       aws.ToString(e.Message),
			LogStreamName: aws.ToString(e.LogStreamName),
		})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].EventID < events[j].EventID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
}
