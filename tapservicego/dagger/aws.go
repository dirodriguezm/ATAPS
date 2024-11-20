package main

import (
	"context"
	"dagger/tapservicego/internal/dagger"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func withAWSCredentials(awsAccessKeyID, awsSecretAccessKey, awsSessionToken *dagger.Secret, ctx context.Context) func(*dagger.Container) *dagger.Container {
	return func(container *dagger.Container) *dagger.Container {
		key, _ := awsAccessKeyID.Plaintext(ctx)
		secret, _ := awsSecretAccessKey.Plaintext(ctx)
		session, _ := awsSessionToken.Plaintext(ctx)

		return container.
			WithEnvVariable("AWS_ACCESS_KEY_ID", key).
			WithEnvVariable("AWS_SECRET_ACCESS_KEY", secret).
			WithEnvVariable("AWS_SESSION_TOKEN", session)
	}
}

func getSsmValue(parameterName string) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	fmt.Print(cfg)
	if err != nil {
		return "", err
	}
	// Create an SSM client
	client := ssm.NewFromConfig(cfg)
	parameter, err := client.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name: &parameterName,
	})
	//fmt.Print(err)
	if err != nil {
		return "", err
	}
	return *parameter.Parameter.Value, nil
}
