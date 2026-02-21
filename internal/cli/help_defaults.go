package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var commandExamples = map[string]string{
	"awstbx": strings.TrimSpace(`
awstbx ec2 list-eips --output json
awstbx cloudwatch delete-log-groups --retention-days 30 --dry-run
awstbx ssm import-parameters --input-file params.json --no-confirm`),
	"awstbx completion": strings.TrimSpace(`
awstbx completion zsh > "${fpath[1]}/_awstbx"
awstbx completion bash > /etc/bash_completion.d/awstbx`),
	"awstbx version": strings.TrimSpace(`
awstbx version
awstbx --version`),
	"awstbx appstream": strings.TrimSpace(`
awstbx appstream delete-image --image-name image-name --dry-run
awstbx appstream delete-image --image-name image-name --no-confirm`),
	"awstbx appstream delete-image": strings.TrimSpace(`
awstbx appstream delete-image --image-name image-name --dry-run
awstbx appstream delete-image --image-name image-name --no-confirm`),
	"awstbx cloudformation": strings.TrimSpace(`
awstbx cloudformation delete-stackset --stackset-name my-stackset --dry-run
awstbx cloudformation find-stack-by-resource --resource i-0123456789abcdef0`),
	"awstbx cloudformation delete-stackset": strings.TrimSpace(`
awstbx cloudformation delete-stackset --stackset-name my-stackset --dry-run
awstbx cloudformation delete-stackset --stackset-name my-stackset --no-confirm`),
	"awstbx cloudformation find-stack-by-resource": strings.TrimSpace(`
awstbx cloudformation find-stack-by-resource --resource i-0123456789abcdef0
awstbx cloudformation find-stack-by-resource --resource AWS::S3::Bucket --include-nested`),
	"awstbx cloudwatch": strings.TrimSpace(`
awstbx cloudwatch count-log-groups
awstbx cloudwatch delete-log-groups --retention-days 30 --filter-name-contains /aws/lambda --dry-run`),
	"awstbx cloudwatch count-log-groups": strings.TrimSpace(`
awstbx cloudwatch count-log-groups
awstbx cloudwatch count-log-groups --output json`),
	"awstbx cloudwatch delete-log-groups": strings.TrimSpace(`
awstbx cloudwatch delete-log-groups --retention-days 30 --dry-run
awstbx cloudwatch delete-log-groups --filter-name-contains /aws/ecs --no-confirm`),
	"awstbx cloudwatch list-log-groups": strings.TrimSpace(`
awstbx cloudwatch list-log-groups
awstbx cloudwatch list-log-groups --output json`),
	"awstbx cloudwatch set-retention": strings.TrimSpace(`
awstbx cloudwatch set-retention --retention-days 30 --dry-run
awstbx cloudwatch set-retention --print-retention-counts`),
	"awstbx ec2": strings.TrimSpace(`
awstbx ec2 list-eips
awstbx ec2 delete-volumes --dry-run`),
	"awstbx ec2 delete-amis": strings.TrimSpace(`
awstbx ec2 delete-amis --retention-days 90 --dry-run
awstbx ec2 delete-amis --unused --no-confirm`),
	"awstbx ec2 delete-eips": strings.TrimSpace(`
awstbx ec2 delete-eips --dry-run
awstbx ec2 delete-eips --no-confirm`),
	"awstbx ec2 delete-keypairs": strings.TrimSpace(`
awstbx ec2 delete-keypairs --dry-run
awstbx ec2 delete-keypairs --all-regions --no-confirm`),
	"awstbx ec2 delete-security-groups": strings.TrimSpace(`
awstbx ec2 delete-security-groups --unused --type ec2 --dry-run
awstbx ec2 delete-security-groups --ssh-rules --no-confirm`),
	"awstbx ec2 delete-snapshots": strings.TrimSpace(`
awstbx ec2 delete-snapshots --retention-days 60 --dry-run
awstbx ec2 delete-snapshots --no-confirm`),
	"awstbx ec2 delete-volumes": strings.TrimSpace(`
awstbx ec2 delete-volumes --dry-run
awstbx ec2 delete-volumes --no-confirm`),
	"awstbx ec2 list-eips": strings.TrimSpace(`
awstbx ec2 list-eips
awstbx ec2 list-eips --output json`),
	"awstbx ecs": strings.TrimSpace(`
awstbx ecs delete-task-definitions --dry-run
awstbx ecs publish-image --ecr-url 123456789012.dkr.ecr.us-east-1.amazonaws.com/app`),
	"awstbx ecs delete-task-definitions": strings.TrimSpace(`
awstbx ecs delete-task-definitions --dry-run
awstbx ecs delete-task-definitions --no-confirm`),
	"awstbx ecs publish-image": strings.TrimSpace(`
awstbx ecs publish-image --ecr-url 123456789012.dkr.ecr.us-east-1.amazonaws.com/app --tag v1.0.0
awstbx ecs publish-image --ecr-url 123456789012.dkr.ecr.us-east-1.amazonaws.com/app --dockerfile ./Dockerfile --context .`),
	"awstbx efs": strings.TrimSpace(`
awstbx efs delete-filesystems --filter-tag Environment=dev --dry-run
awstbx efs delete-filesystems --no-confirm`),
	"awstbx efs delete-filesystems": strings.TrimSpace(`
awstbx efs delete-filesystems --filter-tag Environment=dev --dry-run
awstbx efs delete-filesystems --no-confirm`),
	"awstbx iam": strings.TrimSpace(`
awstbx iam create-sso-users --emails alice@example.com,bob@example.com --group Engineers
awstbx iam rotate-keys --username jdoe`),
	"awstbx iam create-sso-users": strings.TrimSpace(`
awstbx iam create-sso-users --input-file users.txt --group Engineering
awstbx iam create-sso-users --emails alice@example.com,bob@example.com`),
	"awstbx iam delete-user": strings.TrimSpace(`
awstbx iam delete-user --username jdoe --dry-run
awstbx iam delete-user --username jdoe --no-confirm`),
	"awstbx iam rotate-keys": strings.TrimSpace(`
awstbx iam rotate-keys --username jdoe
awstbx iam rotate-keys --username jdoe --key AKIAEXAMPLE --disable`),
	"awstbx kms": strings.TrimSpace(`
awstbx kms delete-keys --unused --pending-days 7 --dry-run
awstbx kms delete-keys --filter-tag Environment=dev --pending-days 30 --no-confirm`),
	"awstbx kms delete-keys": strings.TrimSpace(`
awstbx kms delete-keys --unused --pending-days 7 --dry-run
awstbx kms delete-keys --filter-tag Environment=dev --pending-days 30 --no-confirm`),
	"awstbx org": strings.TrimSpace(`
awstbx org list-accounts --output json
awstbx org generate-diagram --max-accounts-per-ou 8`),
	"awstbx org assign-sso-access": strings.TrimSpace(`
awstbx org assign-sso-access --principal-name Engineering --principal-type GROUP --permission-set-name AdministratorAccess --ou-name Sandbox
awstbx org assign-sso-access --principal-name jane@example.com --principal-type USER --permission-set-name ReadOnlyAccess --ou-name Dev`),
	"awstbx org generate-diagram": strings.TrimSpace(`
awstbx org generate-diagram > org.mmd
awstbx org generate-diagram --max-accounts-per-ou 10`),
	"awstbx org get-account": strings.TrimSpace(`
awstbx org get-account --account-id 123456789012
awstbx org get-account --account-id 123456789012 --output json`),
	"awstbx org import-sso-users": strings.TrimSpace(`
awstbx org import-sso-users --input-file users.csv --dry-run
awstbx org import-sso-users --input-file users.csv --no-confirm`),
	"awstbx org list-accounts": strings.TrimSpace(`
awstbx org list-accounts
awstbx org list-accounts --ou-name Sandbox,Production --output json`),
	"awstbx org list-sso-assignments": strings.TrimSpace(`
awstbx org list-sso-assignments
awstbx org list-sso-assignments --account-id 123456789012`),
	"awstbx org remove-sso-access": strings.TrimSpace(`
awstbx org remove-sso-access --principal-name Engineering --principal-type GROUP --permission-set-name AdministratorAccess --ou-name Sandbox --dry-run
awstbx org remove-sso-access --principal-name Engineering --principal-type GROUP --permission-set-name AdministratorAccess --ou-name Sandbox --no-confirm`),
	"awstbx org set-alternate-contact": strings.TrimSpace(`
awstbx org set-alternate-contact --input-file contacts.json --dry-run
awstbx org set-alternate-contact --input-file contacts.json --no-confirm`),
	"awstbx r53": strings.TrimSpace(`
awstbx r53 create-health-checks --domains example.com,www.example.com --dry-run
awstbx r53 create-health-checks --domains api.example.com --no-confirm`),
	"awstbx r53 create-health-checks": strings.TrimSpace(`
awstbx r53 create-health-checks --domains example.com,www.example.com --dry-run
awstbx r53 create-health-checks --domains api.example.com --no-confirm`),
	"awstbx s3": strings.TrimSpace(`
awstbx s3 search-objects --bucket-name my-bucket --keys invoice.csv,report.json
awstbx s3 delete-buckets --empty --dry-run`),
	"awstbx s3 delete-buckets": strings.TrimSpace(`
awstbx s3 delete-buckets --empty --dry-run
awstbx s3 delete-buckets --filter-name-contains my-bucket --no-confirm`),
	"awstbx s3 download-bucket": strings.TrimSpace(`
awstbx s3 download-bucket --bucket-name my-bucket --prefix exports/ --output-dir ./downloads
awstbx s3 download-bucket --bucket-name my-bucket --prefix logs/`),
	"awstbx s3 list-old-files": strings.TrimSpace(`
awstbx s3 list-old-files --bucket-name my-bucket --older-than-days 90
awstbx s3 list-old-files --bucket-name my-bucket --prefix archive/ --output json`),
	"awstbx s3 search-objects": strings.TrimSpace(`
awstbx s3 search-objects --bucket-name my-bucket --keys foo.txt,bar.txt
awstbx s3 search-objects --bucket-name my-bucket --prefix logs/ --output json`),
	"awstbx sagemaker": strings.TrimSpace(`
awstbx sagemaker cleanup-spaces --domain-id d-abc123 --dry-run
awstbx sagemaker delete-user-profile --domain-id d-abc123 --user-profile data-scientist`),
	"awstbx sagemaker cleanup-spaces": strings.TrimSpace(`
awstbx sagemaker cleanup-spaces --domain-id d-abc123 --spaces studio-default --dry-run
awstbx sagemaker cleanup-spaces --domain-id d-abc123 --no-confirm`),
	"awstbx sagemaker delete-user-profile": strings.TrimSpace(`
awstbx sagemaker delete-user-profile --domain-id d-abc123 --user-profile data-scientist --dry-run
awstbx sagemaker delete-user-profile --domain-id d-abc123 --user-profile data-scientist --no-confirm`),
	"awstbx ssm": strings.TrimSpace(`
awstbx ssm import-parameters --input-file params.json --dry-run
awstbx ssm delete-parameters --input-file params.json --no-confirm`),
	"awstbx ssm delete-parameters": strings.TrimSpace(`
awstbx ssm delete-parameters --input-file params.json --dry-run
awstbx ssm delete-parameters --input-file params.json --no-confirm`),
	"awstbx ssm import-parameters": strings.TrimSpace(`
awstbx ssm import-parameters --input-file params.json --dry-run
awstbx ssm import-parameters --input-file params.json --no-confirm`),
}

func applyCommandHelpDefaults(root *cobra.Command) {
	walkCommands(root, func(cmd *cobra.Command) {
		if strings.TrimSpace(cmd.Long) == "" && strings.TrimSpace(cmd.Short) != "" {
			cmd.Long = cmd.Short
		}

		if strings.TrimSpace(cmd.Example) == "" {
			cmd.Example = defaultCommandExample(cmd)
		}
	})
}

func defaultCommandExample(cmd *cobra.Command) string {
	if example, ok := commandExamples[cmd.CommandPath()]; ok {
		return example
	}

	if cmd.HasAvailableSubCommands() {
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() || sub.Hidden {
				continue
			}
			return strings.TrimSpace(fmt.Sprintf(`
%s --help
%s --help`, cmd.CommandPath(), sub.CommandPath()))
		}
	}

	return strings.TrimSpace(fmt.Sprintf(`
%s --output table
%s --output json`, cmd.CommandPath(), cmd.CommandPath()))
}

func walkCommands(root *cobra.Command, visit func(*cobra.Command)) {
	visit(root)
	for _, child := range root.Commands() {
		walkCommands(child, visit)
	}
}
