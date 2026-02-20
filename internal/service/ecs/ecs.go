package ecs

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
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type API interface {
	DeleteTaskDefinitions(context.Context, *ecs.DeleteTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error)
	ListTaskDefinitions(context.Context, *ecs.ListTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error)
}

type CommandRunner interface {
	Run(context.Context, string, []string, string) (string, error)
}

type systemCommandRunner struct{}

func (systemCommandRunner) Run(ctx context.Context, binary string, args []string, stdin string) (string, error) {
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

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return ecs.NewFromConfig(cfg)
}
var runner CommandRunner = systemCommandRunner{}

func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("ecs", "Manage ECS resources")

	cmd.AddCommand(newDeleteTaskDefinitionsCommand())
	cmd.AddCommand(newPublishImageCommand())

	return cmd
}

func newDeleteTaskDefinitionsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "delete-task-definitions",
		Short:        "Delete inactive ECS task definitions",
		RunE:         runDeleteTaskDefinitions,
		SilenceUsage: true,
	}
}

func newPublishImageCommand() *cobra.Command {
	var ecrURL string
	var dockerfile string
	var imageTag string
	var contextDir string

	cmd := &cobra.Command{
		Use:   "publish-image",
		Short: "Build and publish a Docker image to ECR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPublishImage(cmd, ecrURL, dockerfile, imageTag, contextDir)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&ecrURL, "ecr-url", "", "ECR repository URL")
	cmd.Flags().StringVar(&dockerfile, "dockerfile", "./Dockerfile", "Path to the Dockerfile")
	cmd.Flags().StringVar(&imageTag, "tag", "latest", "Image tag")
	cmd.Flags().StringVar(&contextDir, "context", ".", "Docker build context directory")

	return cmd
}

func runDeleteTaskDefinitions(cmd *cobra.Command, _ []string) error {
	runtime, cfg, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	taskDefinitionARNs, err := listInactiveTaskDefinitionARNs(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list inactive task definitions: %s", awstbxaws.FormatUserError(err))
	}

	sort.Strings(taskDefinitionARNs)

	rows := make([][]string, 0, len(taskDefinitionARNs))
	for _, arn := range taskDefinitionARNs {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{arn, cfg.Region, action})
	}

	return cliutil.RunDestructiveActionPlan(cmd, runtime, cliutil.DestructiveActionPlan{
		Headers:       []string{"task_definition_arn", "region", "action"},
		Rows:          rows,
		ActionColumn:  2,
		ConfirmPrompt: fmt.Sprintf("Delete %d inactive ECS task definition(s)", len(taskDefinitionARNs)),
		Execute: func(rowIndex int) string {
			arn := taskDefinitionARNs[rowIndex]
			output, deleteErr := client.DeleteTaskDefinitions(cmd.Context(), &ecs.DeleteTaskDefinitionsInput{TaskDefinitions: []string{arn}})
			if deleteErr != nil {
				return cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			}
			if len(output.Failures) > 0 {
				return cliutil.FailedActionMessage(failureReason(output.Failures[0]))
			}
			return cliutil.ActionDeleted
		},
	})
}

func runPublishImage(cmd *cobra.Command, ecrURL, dockerfile, imageTag, contextDir string) error {
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

	runtime, err := cliutil.NewCommandRuntime(cmd)
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
		return cliutil.WriteDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
	}

	password, runErr := runner.Run(cmd.Context(), "aws", awsArgs, "")
	if runErr != nil {
		rows[0][2] = cliutil.FailedAction(runErr)
		markRowsSkipped(rows, 1)
		return cliutil.WriteDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
	}

	_, runErr = runner.Run(cmd.Context(), "docker", []string{"login", "--username", "AWS", "--password-stdin", registry}, strings.TrimSpace(password)+"\n")
	if runErr != nil {
		rows[0][2] = cliutil.FailedAction(runErr)
		markRowsSkipped(rows, 1)
		return cliutil.WriteDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
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
		if _, stepErr := runner.Run(cmd.Context(), step.binary, step.args, ""); stepErr != nil {
			rows[step.index][2] = cliutil.FailedAction(stepErr)
			markRowsSkipped(rows, step.index+1)
			return cliutil.WriteDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
		}
		rows[step.index][2] = "completed"
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"step", "command", "action"}, rows)
}

func listInactiveTaskDefinitionARNs(ctx context.Context, client API) ([]string, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[string], error) {
		page, err := client.ListTaskDefinitions(callCtx, &ecs.ListTaskDefinitionsInput{
			NextToken: nextToken,
			Status:    ecstypes.TaskDefinitionStatusInactive,
		})
		if err != nil {
			return awstbxaws.PageResult[string]{}, err
		}
		return awstbxaws.PageResult[string]{
			Items:     page.TaskDefinitionArns,
			NextToken: page.NextToken,
		}, nil
	})
}

func failureReason(failure ecstypes.Failure) string {
	reason := cliutil.PointerToString(failure.Reason)
	detail := cliutil.PointerToString(failure.Detail)
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
			rows[i][2] = cliutil.SkippedActionMessage("previous step failed")
		}
	}
}
