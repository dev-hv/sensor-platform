package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	ssmReadKeyPath  = "/sensor-platform/prod/READ_API_KEY"
	ssmWriteKeyPath = "/sensor-platform/prod/WRITE_API_KEY"
)

// APIKeys holds read and write API keys for request authentication.
type APIKeys struct {
	ReadKey  string
	WriteKey string
}

// ErrAPIKeysNotSet is returned when keys cannot be resolved from SSM or the environment.
var ErrAPIKeysNotSet = errors.New("READ_API_KEY and WRITE_API_KEY must be set")

// GetAPIKeys loads API keys from AWS SSM Parameter Store (SecureString, decrypted).
// On any SSM failure it logs a warning and falls back to READ_API_KEY and WRITE_API_KEY from the environment.
func GetAPIKeys(ctx context.Context) (APIKeys, error) {
	readKey, writeKey, err := fetchAPIKeysFromSSM(ctx)
	if err != nil {
		log.Printf("warning: AWS SSM fetch failed, falling back to local .env")
		readKey = strings.TrimSpace(os.Getenv("READ_API_KEY"))
		writeKey = strings.TrimSpace(os.Getenv("WRITE_API_KEY"))
	}
	if readKey == "" || writeKey == "" {
		return APIKeys{}, ErrAPIKeysNotSet
	}
	return APIKeys{ReadKey: readKey, WriteKey: writeKey}, nil
}

func fetchAPIKeysFromSSM(ctx context.Context) (string, string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", "", fmt.Errorf("load AWS config: %w", err)
	}
	client := ssm.NewFromConfig(cfg)

	readKey, err := getSSMParameter(ctx, client, ssmReadKeyPath)
	if err != nil {
		return "", "", err
	}
	writeKey, err := getSSMParameter(ctx, client, ssmWriteKeyPath)
	if err != nil {
		return "", "", err
	}
	return readKey, writeKey, nil
}

func getSSMParameter(ctx context.Context, client *ssm.Client, name string) (string, error) {
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("get parameter %s: %w", name, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", fmt.Errorf("get parameter %s: empty value", name)
	}
	return strings.TrimSpace(*out.Parameter.Value), nil
}
