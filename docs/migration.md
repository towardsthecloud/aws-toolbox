# Migration Guide: Legacy Scripts to `awstbx`

This guide maps every original automation entry in this repository to its `awstbx` replacement (or current status).

Coverage:
- 57 legacy scripts (49 Python + 8 shell)
- 1 CLI alias file

Total: 58 legacy items.

## Migrated to `awstbx`

| Legacy item | `awstbx` replacement | Notes |
| --- | --- | --- |
| `archived/appstream/appstream_delete_image.py` | `awstbx appstream delete-image` | Direct port |
| `archived/cloudformation/cfn_delete_stackset.py` | `awstbx cfn delete-stackset` | Direct port |
| `archived/cloudformation/cfn_find_stack_by_resource.py` | `awstbx cfn find-stack-by-resource` | Direct port |
| `archived/cloudwatch/cw_count_log_groups.py` | `awstbx cloudwatch count-log-groups` | Direct port |
| `archived/cloudwatch/cw_delete_log_groups.py` | `awstbx cloudwatch delete-log-groups` | Merged command |
| `archived/cloudwatch/cw_delete_log_groups_by_name.py` | `awstbx cloudwatch delete-log-groups --filter-name-contains <text>` | Merged command |
| `archived/cloudwatch/cw_fetch_log_groups_with_creation_date.py` | `awstbx cloudwatch list-log-groups` | Renamed for clarity |
| `archived/cloudwatch/cw_set_retention_policy.py` | `awstbx cloudwatch set-retention` | Direct port |
| `archived/ec2/ec2_delete_unattached_volumes.py` | `awstbx ec2 delete-volumes` | Merged with finder behavior |
| `archived/ec2/ec2_delete_orphaned_snapshots.py` | `awstbx ec2 delete-snapshots` | Direct port |
| `archived/ec2/ec2_delete_old_amis.py` | `awstbx ec2 delete-amis --retention-days <days>` | Merged command |
| `archived/ec2/ec2_delete_ssh_access_security_groups.py` | `awstbx ec2 delete-security-groups --ssh-rules` | Merged command |
| `archived/ec2/ec2_delete_unused_amis.py` | `awstbx ec2 delete-amis --unused` | Merged command |
| `archived/ec2/ec2_delete_unused_eips.py` | `awstbx ec2 delete-eips` | Direct port |
| `archived/ec2/ec2_delete_unused_keypairs_all_regions.py` | `awstbx ec2 delete-keypairs --all-regions` | Merged command |
| `archived/ec2/ec2_delete_unused_keypairs_single_region.py` | `awstbx ec2 delete-keypairs` | Merged command |
| `archived/ec2/ec2_delete_tagged_security_groups.py` | `awstbx ec2 delete-security-groups --filter-tag KEY=VALUE` | Merged command |
| `archived/ec2/ec2_find_unattached_volumes.py` | `awstbx ec2 delete-volumes --dry-run` | Finder merged into dry-run |
| `archived/ec2/ec2_list_available_eips.sh` | `awstbx ec2 list-eips` | Rewritten from shell |
| `archived/ecs/ecs_delete_inactive_task_definitions.py` | `awstbx ecs delete-task-definitions` | Direct port |
| `archived/ecs/ecs_publish_ecr_image.sh` | `awstbx ecs publish-image` | Rewritten from shell |
| `archived/efs/efs_delete_tagged_filesystems.py` | `awstbx efs delete-filesystems` | Direct port |
| `archived/iam/iam_delete_user.py` | `awstbx iam delete-user` | Cascade cleanup included |
| `archived/iam/iam_identity_center_create_users.py` | `awstbx iam create-sso-users` | Direct port |
| `archived/iam/iam_rotate_access_keys.py` | `awstbx iam rotate-keys` | Direct port |
| `archived/kms/kms_delete_keys_by_tag.py` | `awstbx kms delete-keys --filter-tag KEY=VALUE` | Merged command |
| `archived/kms/kms_delete_unused_keys.py` | `awstbx kms delete-keys --unused` | Merged command |
| `archived/organizations/org_assign_sso_access_by_ou.py` | `awstbx org assign-sso-access` | Direct port |
| `archived/organizations/org_generate_mermaid_diagram.py` | `awstbx org generate-diagram` | Direct port |
| `archived/organizations/org_get_account_details.py` | `awstbx org get-account` | Direct port |
| `archived/organizations/org_import_users_to_sso.py` | `awstbx org import-sso-users` | Direct port |
| `archived/organizations/org_list_accounts_by_ou.py` | `awstbx org list-accounts` | Direct port |
| `archived/organizations/org_list_sso_assignments.py` | `awstbx org list-sso-assignments` | Direct port |
| `archived/organizations/org_remove_sso_access_by_ou.py` | `awstbx org remove-sso-access` | Direct port |
| `archived/r53/r53_create_health_checks.py` | `awstbx r53 create-health-checks --domains <list>` | Direct port |
| `archived/s3/s3_delete_empty_buckets.py` | `awstbx s3 delete-buckets --empty` | Merged command |
| `archived/s3/s3_list_old_files.py` | `awstbx s3 list-old-files` | Direct port |
| `archived/s3/s3_search_bucket_and_delete.py` | `awstbx s3 delete-buckets --filter-name-contains <bucket>` | Merged command |
| `archived/s3/s3_search_bucket_and_download.py` | `awstbx s3 download-bucket --bucket-name <bucket> --prefix <prefix>` | Direct port |
| `archived/s3/s3_search_file.py` | `awstbx s3 search-objects --bucket-name <bucket> --keys <key>` | Merged command |
| `archived/s3/s3_search_key.py` | `awstbx s3 search-objects --bucket-name <bucket> --keys <key>` | Merged command |
| `archived/s3/s3_search_multiple_keys.py` | `awstbx s3 search-objects --bucket-name <bucket> --keys <k1,k2>` | Merged command |
| `archived/s3/s3_search_subdirectory.py` | `awstbx s3 search-objects --bucket-name <bucket> --prefix <prefix>` | Merged command |
| `archived/sagemaker/sm_cleanup_spaces.py` | `awstbx sagemaker cleanup-spaces` | Direct port |
| `archived/sagemaker/sm_delete_user_profile.py` | `awstbx sagemaker delete-user-profile` | Direct port |
| `archived/ssm/ssm_delete_parameters.sh` | `awstbx ssm delete-parameters --input-file <file>` | Rewritten from shell |
| `archived/ssm/ssm_import_parameters.sh` | `awstbx ssm import-parameters --input-file <file>` | Rewritten from shell |
| `archived/general/delete_unused_security_groups.py` | `awstbx ec2 delete-security-groups --unused` | Merged into EC2 group command |
| `archived/general/set-alternate-contact.py` | `awstbx org set-alternate-contact --input-file <file>` | Moved under `org` |

## Not Migrated (Dropped)

| Legacy item | Status | Reason |
| --- | --- | --- |
| `archived/codepipeline/cp_slack_notifications.py` | Dropped | Lambda handler, not a CLI workflow |
| `archived/s3/s3_create_tar.py` | Dropped | Lambda + hardcoded assumptions |
| `archived/general/tag_secrets_manager_secrets.py` | Dropped | Lambda + hardcoded tags |
| `archived/general/multi_account_command_executor.py` | Dropped | Lambda + hardcoded account/role model |
| `archived/ec2/ec2_request_spot_instances.sh` | Dropped | Fully hardcoded values |

## Not Migrated (Excluded)

| Legacy item | Status | Reason |
| --- | --- | --- |
| `archived/cli/alias` | Excluded | AWS CLI alias set remains separate from `awstbx` |
| `archived/ec2/ec2_asg_ssh.sh` | Excluded | Interactive SSH helper, outside remote CLI scope |
| `archived/ec2/ec2_resize_volume.sh` | Excluded | Intended to run on-instance (IMDS dependent) |
| `archived/iam/iam_assume_role.sh` | Excluded | Native AWS CLI profile role assumption handles this |
