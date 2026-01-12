package aws

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/skdltmxn/taws/internal/domain"
)

type LambdaClient struct {
	client *lambda.Client
}

func NewLambdaClient(cfg aws.Config) *LambdaClient {
	return &LambdaClient{
		client: lambda.NewFromConfig(cfg),
	}
}

func (c *LambdaClient) ListFunctions(ctx context.Context) ([]domain.LambdaFunction, error) {
	input := &lambda.ListFunctionsInput{}
	var functions []domain.LambdaFunction

	paginator := lambda.NewListFunctionsPaginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list functions: %w", err)
		}

		for _, fn := range output.Functions {
			lastModified := parseLastModified(derefString(fn.LastModified))

			functions = append(functions, domain.LambdaFunction{
				Name:         derefString(fn.FunctionName),
				ARN:          derefString(fn.FunctionArn),
				Runtime:      string(fn.Runtime),
				Handler:      derefString(fn.Handler),
				Description:  derefString(fn.Description),
				Timeout:      derefInt32(fn.Timeout),
				MemorySize:   derefInt32(fn.MemorySize),
				CodeSize:     fn.CodeSize,
				State:        domain.FunctionState(fn.State),
				LastModified: lastModified,
				Role:         derefString(fn.Role),
			})
		}
	}

	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Name == functions[j].Name {
			return functions[i].ARN < functions[j].ARN
		}
		return functions[i].Name < functions[j].Name
	})

	return functions, nil
}

func (c *LambdaClient) DeleteFunction(ctx context.Context, functionName string) error {
	_, err := c.client.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete function %s: %w", functionName, err)
	}
	return nil
}

func parseLastModified(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02T15:04:05.000+0000", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (c *LambdaClient) GetFunctionCode(ctx context.Context, functionName string) ([]domain.LambdaCodeFile, error) {
	output, err := c.client.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get function %s: %w", functionName, err)
	}

	if output.Code == nil || output.Code.Location == nil {
		return nil, fmt.Errorf("no code location available for function %s", functionName)
	}

	resp, err := http.Get(*output.Code.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to download code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download code: HTTP %d", resp.StatusCode)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read code: %w", err)
	}

	return extractCodeFiles(zipData)
}

func extractCodeFiles(zipData []byte) ([]domain.LambdaCodeFile, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}

	var files []domain.LambdaCodeFile
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		if !isTextFile(file.Name) {
			continue
		}

		if file.UncompressedSize64 > 1024*1024 {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		files = append(files, domain.LambdaCodeFile{
			Path:    file.Name,
			Content: string(content),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}

func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	textExts := map[string]bool{
		".py": true, ".js": true, ".ts": true, ".mjs": true, ".cjs": true,
		".java": true, ".go": true, ".rb": true, ".cs": true,
		".json": true, ".yaml": true, ".yml": true, ".xml": true,
		".txt": true, ".md": true, ".sh": true, ".bash": true,
		".html": true, ".css": true, ".sql": true, ".rs": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true,
	}
	if textExts[ext] {
		return true
	}

	base := strings.ToLower(filepath.Base(name))
	specialFiles := map[string]bool{
		"dockerfile": true, "makefile": true, "requirements.txt": true,
		"package.json": true, "index.js": true, "handler.py": true,
	}
	return specialFiles[base]
}
