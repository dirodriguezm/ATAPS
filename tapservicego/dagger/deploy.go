package main

import (
	"context"
	"dagger/tapservicego/internal/dagger"
	"fmt"
	"log"
	"strings"
)

// Publish the tap service Helm chart to GHCR
func (m *Tapservicego) PublishHelmChart(
	ctx context.Context,
	chartDir *dagger.Directory,
	username string,
	password *dagger.Secret,
	awsAccessKeyID *dagger.Secret,
	awsSecretAccessKey *dagger.Secret,
	awsSessionToken *dagger.Secret,
	ghOrg *string,
) (string, error) {
	container := dag.Container().
		From("alpine/k8s:1.31.0").
		WithDirectory("/usr/src/chart", chartDir).
		WithExec([]string{"helm", "package", "/usr/src/chart", "-d", "/usr/src"})
	version, err := container.
		WithExec([]string{"sh", "-c", "grep '^version:' /usr/src/chart/Chart.yaml | awk '{print $2}' | tr -d '\"'"}).
		Stdout(ctx)
	version = strings.Trim(version, "\n")
	if err != nil {
		log.Fatalf("Error getting chart version: %v", err)
	}
	pwd, err := password.Plaintext(ctx)
	if err != nil {
		log.Fatalf("Error getting password: %v", err)
	}
	if ghOrg == nil {
		ghOrg = &username
	}
	registry := fmt.Sprintf("oci://ghcr.io/%s/tapservice-chart", *ghOrg)
	return container.
		With(withAWSCredentials(awsAccessKeyID, awsSecretAccessKey, awsSessionToken, ctx)).
		WithExec([]string{"helm", "registry", "login", "-u", username, "-p", pwd, "ghcr.io"}).
		WithExec([]string{"helm", "push", fmt.Sprintf("/usr/src/tapservice-%s.tgz", version), registry}).
		Stdout(ctx)
}

// Deploy the tap service Helm chart
func (m *Tapservicego) Deploy(
	ctx context.Context,
	username string,
	password *dagger.Secret,
	awsAccessKeyID *dagger.Secret,
	awsSecretAccessKey *dagger.Secret,
	awsSessionToken *dagger.Secret,
	chartUrl string,
	helmValues *string,
	dryRun bool,
) *dagger.Container {
	pwd, err := password.Plaintext(ctx)
	if err != nil {
		log.Fatalf("Error getting password: %v", err)
	}
	m.ChartUrl = chartUrl
	if err != nil {
		log.Fatalf("Error getting helm values file: %v", err)
	}
	m.HelmValuesSource = helmValues
	m.DryRun = dryRun
	return dag.Container().
		From("alpine/k8s:1.31.0").
		With(withAWSCredentials(awsAccessKeyID, awsSecretAccessKey, awsSessionToken, ctx)).
		With(m.helmValuesFile).
		WithWorkdir("/usr/src/app").
		WithExec([]string{"helm", "registry", "login", "-u", username, "-p", pwd, "ghcr.io"}).
		With(m.upgradeCommand(ctx))
}

func (m *Tapservicego) upgradeCommand(ctx context.Context) func(container *dagger.Container) *dagger.Container {
	return func(container *dagger.Container) *dagger.Container {
		entries, err := container.Directory("/usr/src/app").Entries(ctx)
		if err != nil {
			log.Fatalf("Error getting directory entries: %v", err)
		}
		valuesExists := findInList(entries, "values.yaml")
		command := []string{"helm", "upgrade", "--install", "tapservice", m.ChartUrl}
		if valuesExists {
			command = append(command, "--values", "values.yaml")
		}
		if m.DryRun {
			command = append(command, "--dry-run")
		}
		return container.WithExec(command)
	}
}

func (m *Tapservicego) helmValuesFile(container *dagger.Container) *dagger.Container {
	if m.HelmValuesSource == nil {
		return container
	}
	sourcePrefix := strings.Split(*m.HelmValuesSource, ":")[0]
	switch sourcePrefix {
	case "ssm":
		parameterName := strings.TrimPrefix(*m.HelmValuesSource, "ssm:")
		value, err := getSsmValue(parameterName)
		if err != nil {
			log.Fatalf("Error getting SSM parameter: %v", err)
		}
		return writeValuesToFile(container, value)
	case "file":
		log.Fatalf("File source not yet implemented")
	default:
		log.Fatalf("Invalid helm values source: %s", *m.HelmValuesSource)
	}
	return container
}

func writeValuesToFile(container *dagger.Container, values string) *dagger.Container {
	file := "/usr/src/app/values.yaml"
	return container.WithNewFile(file, values, dagger.ContainerWithNewFileOpts{
		Permissions: 0644,
	})
}

func findInList(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
