# Changelog

All notable changes to this project are documented in this file.

## Unreleased


### Bug Fixes

- Refactor iam scripts and fix flake8 and black format (2ab1a76)

- Typo in readme (73a4fd9)

- Add error handling for security groups that have dependencies (de25a59)

- **ec2:** Simplify Elastic IP release logic (bbce909)

- Correct command for installing dependencies in lint workflow (d36c108)

- Address PR feedback for s3 download safety and org parent output (fa7ec9c)


### Features

- Add delete empty s3 bucket script (21a7c01)

- Initial try at deleting buckets wit ha specific name (9a51983)

- Rename s3 script and update readme with new script (4cb861b)

- Add additional ec2 scripts and update readme (89c7291)

- Add security group scripts (b3874bd)

- Add script that deletes unused elastic ips (441e011)

- Add 2 new scripts (c0a51cc)

- Add script to delete all inactive task definitions (7e64278)

- Improve cw logs script naming (a945f8c)

- Add new aws iam identity center scripts (3e3b42b)

- Add script to remove permission set from ou (5238232)

- Add script with ability to import users from csv to sso (70ff9fb)

- Add ability to find all accounts in organization when you dont add an ou name (b7b20e9)

- Add aws iam action website (bce9d7c)

- Add s3 scripts (38b25c7)

- Add scripts to delete tagged SGs and EFS (be9bf71)

- Reorganize sections in readme and update gh banner (8a3f04b)

- Add script to delete CloudWatch log groups based on age. (c77afb0)

- Add script to unshare and delete AppStream images (16582f2)

- Add scripts for CloudWatch and IAM actions (c654937)

- **docs:** Update README with new EC2 scripts (e410fc9)

- **aws:** Add script to set alternate contacts for organization accounts (6830204)

- **s3:** Add script to download S3 bucket contents (a0cafea)

- **sagemaker:** Add scripts for managing SageMaker spaces and user profiles (36eee3c)

- **security_groups:** Enhance script to identify and delete unused security groups using ENI-based detection and improved type filtering (62c8320)

- **ec2:** Add script to deregister old AMIs and delete associated snapshots (ab1817a)

- Add scripts to find CloudFormation stacks and display AWS organization structure (b93eb50)

- Add scripts for scheduling deletion of KMS keys based on tags and usage (617bf6f)

- Add CUDly tool to Miscellaneous section (3e6824f)

- Add script to delete CloudWatch log groups by name with dry-run support (75a04a4)

- Add CloudBurn tool for automatic AWS cost estimates in FinOps section (731e480)

- Add AWS FinOps Dashboard to FinOps section in README (7cf12c8)

- Add plan to migrate to cli (7edb0b6)

- Scaffolding (485e125)

- Milestone 2 (eb27c5c)

- Milestone 3 (ef4b5ec)

- Mileston 4 (8dceac7)

- Milestone 5 (bfdf39d)

- Milestone 6 (6b23452)

- Remove integration tests and LocalStack configuration (8f11c77)

- Refactor organization commands to improve input handling and streamline AWS client initialization (e9cfd06)

- Add internal/cliutil shared runtime package (f77f15e)


### Other

- Initial commit (2f38962)

- Setup github templates + init scripts. (97f69f2)

- Updated header image. (4284fe5)

- Added cloudwatch logs retention script + added several useful links. (7c6bf3f)

- Update README.md (3beffca)

- Update README.md (c1ace52)

- Merge pull request #2 from andreacavagna01/patch-1

Added Cloud access management tool (9ab2fe5)

- Sorted the tools list and updated the intro. (0c03562)

- Added useful AWS CLI aliases. (f30db62)

- Added a couple of handy tools for EC2, Lambda and CDK. (5563b56)

- Create FUNDING.yml (39cf406)

- Added blogroll & reorganized the scripts. (0e2521c)

- Fix icon size. (d377478)

- Added Former2 tool for IaC. (58a5993)

- Added IAM assume role script. (cc95d0c)

- Add script for requesting spot instances (3a06c52)

- Merge pull request #3 from pavledjuric/pavle

add script for requesting spot instances (7aeae2a)

- Added script to resize EC2 Instances (55b8c57)

- Merge pull request #6 from umegbewe/main

Added script to resize EC2 Instance volumes (69c9156)

- Added S3 scripts

S3 related operation which generally come while doing S3 operation such listing object, searching object, older than N number days etc. (24e7cab)

- Created tar file creation in S3 (e004bb4)

- Created finding file older than days (dc5cf30)

- Search file in S3 bucket (6377e6e)

- Search subdirectory in nested dir (7e8b0b0)

- Update README.md (0cf8e61)

- Update create-tar-file-s3.py (101fbca)

- Merge pull request #5 from aviboy2006/s3-helpful-scripts

Added S3 scripts (1c1f653)

- Updated readme with new scripts and made small fixes. (8e59e5e)

- Moved jq message (65e2d4f)

- Added IaC, IAM and generic tools to the list. (2f5bfb5)

- Added useful SSM tools (312e462)

- Added IAM Floyd to the list. (b3ae8af)

- Adding Lint Action on push to any branch and fixed linting errors. (00c3c1b)

- Added linting + formatting instructions in the contribution guidelines. (5e4e3cb)

- Added a link to: AWS Security Tools repo. (85409b9)

- Update README.md (a2e6b21)

- Added script to tag secrets quickly. (fc9d250)

- Updated introduction and script name in readme. (bf6dc25)

- Moved secret tag script in general, renamed multi-account script and added codepipline_slack_notifcation lambda. (046b1ab)

- Enable lint workflow trigger on PR's. (131f863)

- Added a shell script to publish a Docker image to an ECR repository (c681b44)

- Merge pull request #8 from madagra/main

Added a shell script to publish a Docker image to an ECR repository (03c7e8d)

- Renamed ecs script to allign with naming convention. (52c8c8d)

- Updated domain. (cd9cb81)

- Error: This action does not have permission to create annotations on forks. (b3f8e71)

- Adding script for iam key rotation (367a7e2)

- Merge pull request #9 from santabhai/main

Adding script for iam key rotation (cafff20)

- Adding script to delete iam user (9407409)

- Merge pull request #11 from santabhai/iam

adding script to delete iam user (3485ccb)

- Formatted iam scripts and updated readme. (26f7247)

- Simplify intro. (b28c1db)

- Update tools section name. (0b5800e)

- Adding contributors to readme (7a4afba)

- Suggesting ECS Compose-X for AWS ECS (0182de2)

- Merge pull request #14 from compose-x/JohnPreston/ecs-compose-x

Suggesting ECS Compose-X for AWS ECS (a849ed2)

- Adding script that finds all unused keypairs (2778624)

- Merge pull request #13 from santabhai/patch-2

adding script that finds all unused keypairs (42c711f)

- Adding script that list all unattached volumes (9ed24d5)

- Merge pull request #12 from santabhai/patch-1

Adding script that list all unattached volumes (c245699)

- Merge pull request #18 from dannysteenman/feat/delete-bucket-containing-name

Feat/delete-bucket-containing-name (a1096f6)

- Merge pull request #19 from dannysteenman/feat/ec2-scripts

feat: ec2-scripts (247c802)

- Merge pull request #20 from dannysteenman/feat/security-group-scripts

feat: add security group scripts (f957ad3)

- Merge pull request #21 from dannysteenman/feat/new-scripts

feat: new-scripts (55919fa)

- Merge pull request #22 from dannysteenman/feat/new-scripts-2

feat: add 2 new scripts (35320c3)

- Merge pull request #23 from dannysteenman/feat/renaming-scripts

chore: renamed scripts and updated readme (f647a20)

- Merge pull request #24 from dannysteenman/feat/sso-scripts

feat: add new aws iam identity center scripts (75612c5)

- Add ssm-supercharged to the SSM tools (36340d9)

- Merge pull request #26 from HQarroum/feature/add-ssm-supercharged

Add ssm-supercharged to the SSM tools (414a12b)

- Update set_cloudwatch_logs_retention.py

it should be 'nextToken' (9bebe4e)

- Merge pull request #27 from scari/patch-1

Update set_cloudwatch_logs_retention.py (fcaaa00)

- Merge pull request #30 from towardsthecloud/fix/gh-25

feat(security_groups): enhance unused security group detection with ENI-based approach (249766a)

- Update README.md (cefae8b)

- Update README.md (f20f728)

- Update README.md (90d67c8)

- Update README.md (8f2de65)

- Update README.md (82f5896)

- Update README.md (9d49d38)

- Update README.md (691886e)

- Update README.md (8c46c7a)

- Update README.md (caa1090)

- Merge pull request #32 from cristim/add-cudly-tool

feat: Add CUDly tool to Miscellaneous section (cb96eb9)


### Refactoring

- **ecs:** Improve task definition deletion logic (e9bcc09)

- Rename scripts (5030947)

- Migrate appstream to internal/service/appstream (6015c99)

- Migrate all services to per-package structure under internal/service/ (0378cac)

