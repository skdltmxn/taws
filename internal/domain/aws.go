package domain

import "context"

// AWSIdentity represents the identity of the current user/role
type AWSIdentity struct {
	AccountID string
	Arn       string
	UserID    string
}

// AWSClient defines the interface for AWS interactions
type AWSClient interface {
	// VerifyCredentials checks if the current credentials are valid and returns the identity
	VerifyCredentials(ctx context.Context) (*AWSIdentity, error)
	// EC2 returns the EC2 client
	EC2() EC2Client
	// S3 returns the S3 client
	S3() S3Client
	// VPC returns the VPC client
	VPC() VPCClient
	// EKS returns the EKS client
	EKS() EKSClient
	// ECR returns the ECR client
	ECR() ECRClient
	// IAM returns the IAM client
	IAM() IAMClient
	// Route53 returns the Route53 client
	Route53() Route53Client
	// CloudWatch returns the CloudWatch client
	CloudWatch() CloudWatchClient
	// Lambda returns the Lambda client
	Lambda() LambdaClient
}
