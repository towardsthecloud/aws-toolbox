"""
Description: This script identifies and optionally deletes unused security groups in an AWS account.
It fetches security groups based on the specified type (EC2, RDS, ELB, or all), determines which ones
are currently in use by querying Elastic Network Interfaces (ENIs), and identifies the unused security groups.
The script can perform a dry run to show which security groups would be deleted without actually deleting them.

Key features:
- Uses ENI-based detection for comprehensive coverage of all AWS services
- Supports filtering of security groups by type (EC2, RDS, ELB, or all) based on naming conventions
- Automatically uses the region specified in the AWS CLI profile
- Supports dry run mode for safe execution
- Provides detailed logging of all operations
- Implements error handling for robustness
- Skips deletion of security groups with 'default' in their name

Usage:
python delete_unused_security_groups.py [--dry-run] [--type {all,ec2,rds,elb}]

Arguments:
--dry-run            Perform a dry run without deleting security groups
--type {all,ec2,rds,elb}  Specify the type of security groups to consider based on naming conventions:
                     - all: All security groups
                     - ec2: Security groups not starting with 'rds-' or 'elb-'
                     - rds: Security groups starting with 'rds-'
                     - elb: Security groups starting with 'elb-'

Important Note about Type Filtering:
When a specific type is selected (e.g., --type rds), the script:
1. Filters security groups by naming convention (e.g., only considers groups starting with 'rds-')
2. Still checks if these groups are in use by ANY AWS service (not just RDS)
This ensures safety - a security group with an 'rds-' prefix that's actually used by an EC2 instance
will not be deleted, which is the correct behavior.

The script performs the following steps:
1. Retrieves all security groups of the specified type (based on naming conventions)
2. Identifies security groups in use by querying all Elastic Network Interfaces (ENIs)
3. Determines unused security groups by comparing filtered groups to those in use
4. Deletes unused security groups (unless in dry-run mode)

Note: This script requires appropriate AWS permissions to describe and delete security groups,
as well as to describe network interfaces.

Author: Danny Steenman
License: MIT
"""

import argparse
import logging

import boto3
from botocore.exceptions import ClientError


def setup_logging():
    """Configure logging for the script."""
    logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
    return logging.getLogger(__name__)


def get_used_security_groups(ec2, logger):
    """Collect all security groups in use by querying ENIs."""
    used_sg = set()

    try:
        # Get all ENIs and their attached security groups
        paginator = ec2.get_paginator('describe_network_interfaces')
        for page in paginator.paginate():
            for eni in page["NetworkInterfaces"]:
                for sg in eni["Groups"]:
                    used_sg.add(sg["GroupId"])

        logger.info(f"Found {len(used_sg)} security groups in use across all ENIs")
    except ClientError as e:
        logger.error(f"Error describing network interfaces: {str(e)}")

    return used_sg


def get_all_security_groups(ec2, sg_type, logger):
    """Get all security groups in the region based on the specified type."""
    all_sg = set()
    try:
        paginator = ec2.get_paginator('describe_security_groups')
        for page in paginator.paginate():
            for sg in page["SecurityGroups"]:
                group_name = sg["GroupName"].lower()
                if sg_type == "all":
                    all_sg.add(sg["GroupId"])
                elif sg_type == "ec2" and not (group_name.startswith("rds-") or group_name.startswith("elb-")):
                    all_sg.add(sg["GroupId"])
                elif sg_type == "rds" and group_name.startswith("rds-"):
                    all_sg.add(sg["GroupId"])
                elif sg_type == "elb" and group_name.startswith("elb-"):
                    all_sg.add(sg["GroupId"])
    except ClientError as e:
        logger.error(f"Error describing security groups: {str(e)}")
    return all_sg


def delete_unused_security_groups(ec2, unused_sg, dry_run, logger):
    """Delete unused security groups, skipping those with 'default' in the name."""
    for sg_id in unused_sg:
        try:
            sg_info = ec2.describe_security_groups(GroupIds=[sg_id])["SecurityGroups"][0]
            sg_name = sg_info["GroupName"]

            if "default" in sg_name.lower():
                logger.info(
                    f"Skipping deletion of security group '{sg_name}' (ID: {sg_id}) because it contains 'default'"
                )
                continue

            if dry_run:
                logger.info(f"[DRY RUN] Would delete security group '{sg_name}' (ID: {sg_id})")
            else:
                logger.info(f"Deleting security group '{sg_name}' (ID: {sg_id})")
                ec2.delete_security_group(GroupId=sg_id)
        except ClientError as e:
            if e.response["Error"]["Code"] == "DependencyViolation":
                logger.warning(
                    f"Skipping deletion of security group '{sg_name}' (ID: {sg_id}) because it has a dependent object."
                )
            else:
                logger.error(f"Error deleting security group '{sg_name}' (ID: {sg_id}): {str(e)}")


def main(dry_run, sg_type):
    logger = setup_logging()

    # Initialize AWS client (only need EC2 now)
    ec2 = boto3.client("ec2")

    used_sg = get_used_security_groups(ec2, logger)
    all_sg = get_all_security_groups(ec2, sg_type, logger)
    unused_sg = all_sg - used_sg

    logger.info(f"Total Security Groups ({sg_type}): {len(all_sg)}")
    logger.info(f"Used Security Groups ({sg_type}): {len(used_sg)}")
    logger.info(f"Unused Security Groups ({sg_type}): {len(unused_sg)}")
    logger.info(f"Unused Security Group IDs: {list(unused_sg)}\n")

    if dry_run:
        logger.info("Running in dry-run mode. No security groups will be deleted.")

    delete_unused_security_groups(ec2, unused_sg, dry_run, logger)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Delete unused AWS security groups")
    parser.add_argument("--dry-run", action="store_true", help="Perform a dry run without deleting security groups")
    parser.add_argument(
        "--type",
        choices=["all", "ec2", "rds", "elb"],
        default="all",
        help="Specify the type of security groups to consider based on naming conventions: "
             "- all: All security groups "
             "- ec2: Security groups not starting with 'rds-' or 'elb-' "
             "- rds: Security groups starting with 'rds-' "
             "- elb: Security groups starting with 'elb-'",
    )
    args = parser.parse_args()

    main(args.dry_run, args.type)
