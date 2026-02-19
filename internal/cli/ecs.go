package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type ecsAPI interface {
	DeleteTaskDefinitions(context.Context, *ecs.DeleteTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error)
	ListTaskDefinitions(context.Context, *ecs.ListTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error)
}

type ecsCommandRunner interface {
	Run(context.Context, string, []string, string) (string, error)
}

type systemECSCommandRunner struct{}

func (systemECSCommandRunner) Run(ctx context.Context, binary string, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", binary, strings.Join(args, " "), message)
	}

	return strings.TrimSpace(stdout.String()), nil
}

var ecsLoadAWSConfig = awstbxaws.LoadAWSConfig
var ecsNewClient = func(cfg awssdk.Config) ecsAPI {
	return ecs.NewFromConfig(cfg)
}
var ecsRunner ecsCommandRunner = systemECSCommandRunner{}

func newECSCommand() *cobra.Command {
	cmd := newServiceGroupCommand("ecs", "Manage ECS resources")

	cmd.AddCommand(newECSDeleteTaskDefinitionsCommand())
	cmd.AddCommand(newECSPublishImageCommand())

	return cmd
}

func newECSDeleteTaskDefinitionsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "delete-task-definitions",
		Short:        "Delete inactive ECS task definitions",
		RunE:         runECSDeleteTaskDefinitions,
		SilenceUsage: true,
	}
}

func newECSPublishImageCommand() *cobra.Command {
	var ecrURL string
	var dockerfile string
	var imageTag string
	var contextDir string

	cmd := &cobra.Command{
		Use:   "publish-image",
		Short: "Build and publish a Docker image to ECR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runECSPublishImage(cmd, ecrURL, dockerfile, imageTag, contextDir)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&ecrURL, "ecr-url", "", "ECR repository URL")
	cmd.Flags().StringVar(&dockerfile, "dockerfile", "./Dockerfile", "Path to the Dockerfile")
	cmd.Flags().StringVar(&imageTag, "tag", "latest", "Image tag")
	cmd.Flags().StringVar(&contextDir, "context", ".", "Docker build context directory")

	return cmd
}

func runECSDeleteTaskDefinitions(cmd *cobra.Command, _ []string) error {
	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := ecsLoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := ecsNewClient(cfg)

	taskDefinitionARNs, err := listInactiveTaskDefinitionARNs(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list inactive task definitions: %s", awstbxaws.FormatUserError(err))
	}

	sort.Strings(taskDefinitionARNs)

	rows := make([][]string, 0, len(taskDefinitionARNs))
	for _, arn := range taskDefinitionARNs {
		action := "would-delete"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{arn, cfg.Region, action})
	}

	if len(taskDefinitionARNs) == 0 || runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"task_definition_arn", "region", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Delete %d inactive ECS task definition(s)", len(taskDefinitionARNs)),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			rows[i][2] = "cancelled"
		}
		return writeDataset(cmd, runtime, []string{"task_definition_arn", "region", "action"}, rows)
	}

	for i, arn := range taskDefinitionARNs {
		output, deleteErr := client.DeleteTaskDefinitions(cmd.Context(), &ecs.DeleteTaskDefinitionsInput{TaskDefinitions: []string{arn}})
		if deleteErr != nil {
			rows[i][2] = "failed: " + awstbxaws.FormatUserError(deleteErr)
			continue
		}
		if len(output.Failures) > 0 {
			rows[i][2] = "failed: " + ecsFailureReason(output.Failures[0])
			continue
		}
		rows[i][2] = "deleted"
	}

	return writeDataset(cmd, runtime, []string{"task_definition_arn", "region", "action"}, rows)
}

func runECSPublishImage(cmd *cobra.Command, ecrURL, dockerfile, imageTag, contextDir string) error {
	repoURL := strings.TrimSpace(ecrURL)
	if repoURL == "" {
		return fmt.Errorf("--ecr-url is required")
	}
	if strings.TrimSpace(dockerfile) == "" {
		return fmt.Errorf("--dockerfile is required")
	}
	if strings.TrimSpace(imageTag) == "" {
		return fmt.Errorf("--tag is required")
	}
	if strings.TrimSpace(contextDir) == "" {
		return fmt.Errorf("--context is required")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	registry := repoURL
	if strings.Contains(registry, "/") {
		registry = strings.SplitN(registry, "/", 2)[0]
	}

	localImageRef := fmt.Sprintf("awstbx-ecs-publish:%s", strings.ReplaceAll(strings.TrimSpace(imageTag), "/", "-"))
	remoteImageRef := fmt.Sprintf("%s:%s", repoURL, imageTag)

	awsArgs := make([]string, 0, 6)
	if runtime.Options.Profile != "" {
		awsArgs = append(awsArgs, "--profile", runtime.Options.Profile)
	}
	if runtime.Options.Region != "" {
		awsArgs = append(awsArgs, "--region", runtime.Options.Region)
	}
	awsArgs = append(awsArgs, "ecr", "get-login-password")

	buildArgs := []string{"build", "-t", localImageRef, "-f", dockerfile, contextDir}
	tagArgs := []string{"tag", localImageRef, remoteImageRef}
	pushArgs := []string{"push", remoteImageRef}

	rows := [][]string{
		{"login", "docker login --username AWS --password-stdin " + registry, "pending"},
		{"build", "docker " + strings.Join(buildArgs, " "), "pending"},
		{"tag", "docker " + strings.Join(tagArgs, " "), "pending"},
		{"push", "docker " + strings.Join(pushArgs, " "), "pending"},
	}
	if runtime.Options.DryRun {
		for i := range rows {
			rows[i][2] = "would-run"
		}
		return writeDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
	}

	password, runErr := ecsRunner.Run(cmd.Context(), "aws", awsArgs, "")
	if runErr != nil {
		rows[0][2] = "failed: " + runErr.Error()
		markRowsSkipped(rows, 1)
		return writeDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
	}

	_, runErr = ecsRunner.Run(cmd.Context(), "docker", []string{"login", "--username", "AWS", "--password-stdin", registry}, strings.TrimSpace(password)+"\n")
	if runErr != nil {
		rows[0][2] = "failed: " + runErr.Error()
		markRowsSkipped(rows, 1)
		return writeDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
	}
	rows[0][2] = "completed"

	steps := []struct {
		index  int
		binary string
		args   []string
	}{
		{index: 1, binary: "docker", args: buildArgs},
		{index: 2, binary: "docker", args: tagArgs},
		{index: 3, binary: "docker", args: pushArgs},
	}

	for _, step := range steps {
		if _, stepErr := ecsRunner.Run(cmd.Context(), step.binary, step.args, ""); stepErr != nil {
			rows[step.index][2] = "failed: " + stepErr.Error()
			markRowsSkipped(rows, step.index+1)
			return writeDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
		}
		rows[step.index][2] = "completed"
	}

	return writeDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
}

func listInactiveTaskDefinitionARNs(ctx context.Context, client ecsAPI) ([]string, error) {
	arns := make([]string, 0)
	var nextToken *string

	for {
		page, err := client.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			NextToken: nextToken,
			Status:    ecstypes.TaskDefinitionStatusInactive,
		})
		if err != nil {
			return nil, err
		}

		arns = append(arns, page.TaskDefinitionArns...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return arns, nil
}

func ecsFailureReason(failure ecstypes.Failure) string {
	reason := pointerToString(failure.Reason)
	detail := pointerToString(failure.Detail)
	if reason == "" {
		reason = "unknown failure"
	}
	if detail == "" {
		return reason
	}
	return reason + ": " + detail
}

func markRowsSkipped(rows [][]string, start int) {
	for i := start; i < len(rows); i++ {
		if rows[i][2] == "pending" {
			rows[i][2] = "skipped"
		}
	}
}
