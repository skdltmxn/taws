package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"github.com/skdltmxn/taws/internal/domain"
)

const (
	defaultRegion = "us-east-1"

	TimeoutShort  = 10 * time.Second
	TimeoutMedium = 30 * time.Second
	TimeoutLong   = 5 * time.Minute
)

func extractNameTag(tags []types.Tag) string {
	for _, tag := range tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func derefInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

type Client struct {
	cfg     aws.Config
	profile string
}

// NewClient creates a new AWS client instance
// It tries to load configuration from the environment, shared config, and shared credentials.
// Supports AWS SSO profiles when configured in ~/.aws/config
func NewClient(ctx context.Context, region string, profile string) (*Client, error) {
	opts := []func(*config.LoadOptions) error{
		// Enable loading from ~/.aws/config (required for SSO profiles)
		config.WithSharedConfigFiles([]string{
			config.DefaultSharedConfigFilename(),
		}),
		config.WithSharedCredentialsFiles([]string{
			config.DefaultSharedCredentialsFilename(),
		}),
		// Set default region as fallback
		config.WithDefaultRegion(defaultRegion),
		// Silence AWS SDK logging so it doesn't pollute the TUI.
		config.WithLogger(logging.Nop{}),
	}

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Double-check region is set
	if cfg.Region == "" {
		cfg.Region = defaultRegion
	}

	return &Client{cfg: cfg, profile: profile}, nil
}

// VerifyCredentials checks if the credentials are valid by calling STS GetCallerIdentity
// If SSO token is expired, it attempts to refresh by running 'aws sso login'
func (c *Client) VerifyCredentials(ctx context.Context) (*domain.AWSIdentity, error) {
	svc := sts.NewFromConfig(c.cfg)
	identity, err := svc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		if isSSOError(err) {
			if loginErr := c.runSSOLogin(ctx); loginErr != nil {
				return nil, fmt.Errorf("SSO login failed: %w (original error: %v)", loginErr, err)
			}
			// Reload config after SSO login
			newClient, reloadErr := NewClient(ctx, c.cfg.Region, c.profile)
			if reloadErr != nil {
				return nil, fmt.Errorf("failed to reload config after SSO login: %w", reloadErr)
			}
			c.cfg = newClient.cfg
			// Retry GetCallerIdentity
			svc = sts.NewFromConfig(c.cfg)
			identity, err = svc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
			if err != nil {
				return nil, fmt.Errorf("failed to verify credentials after SSO login: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to verify credentials: %w", err)
		}
	}

	return &domain.AWSIdentity{
		AccountID: *identity.Account,
		Arn:       *identity.Arn,
		UserID:    *identity.UserId,
	}, nil
}

// isSSOError checks if the error is related to SSO authentication
func isSSOError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "sso") ||
		strings.Contains(errStr, "token") ||
		strings.Contains(errStr, "expired") ||
		strings.Contains(errStr, "refresh") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "access denied")
}

// runSSOLogin executes 'aws sso login' command to refresh SSO credentials
func (c *Client) runSSOLogin(ctx context.Context) error {
	args := []string{"sso", "login"}
	if c.profile != "" {
		args = append(args, "--profile", c.profile)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ensure AWS config is loaded (required for SSO on all platforms)
	cmd.Env = append(os.Environ(), "AWS_SDK_LOAD_CONFIG=1")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed: %w", err)
	}

	return nil
}

func (c *Client) EC2() domain.EC2Client {
	return NewEC2Client(c.cfg)
}

func (c *Client) S3() domain.S3Client {
	return NewS3Client(c.cfg)
}

func (c *Client) VPC() domain.VPCClient {
	return NewVPCClient(c.cfg)
}

func (c *Client) EKS() domain.EKSClient {
	return NewEKSClient(c.cfg)
}

func (c *Client) ECR() domain.ECRClient {
	return NewECRClient(c.cfg)
}

func (c *Client) IAM() domain.IAMClient {
	return NewIAMClient(c.cfg)
}

func (c *Client) Route53() domain.Route53Client {
	return NewRoute53Client(c.cfg)
}

func (c *Client) CloudWatch() domain.CloudWatchClient {
	return NewCloudWatchClient(c.cfg)
}

func (c *Client) Lambda() domain.LambdaClient {
	return NewLambdaClient(c.cfg)
}
