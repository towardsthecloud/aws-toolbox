"""
Description: This script deregisters AMIs older than a specified number of days and deletes their associated EBS snapshots.
The script can perform a dry run to show which AMIs would be deregistered without actually doing so.

Key features:
- Automatically uses the region specified in the AWS CLI profile.
- Supports dry-run mode for safe execution.
- Provides detailed logging of all operations.
- Uses boto3 to interact with the AWS EC2 service.
- Allows setting a retention period for AMIs.
- Deletes associated EBS snapshots upon deregistering an AMI.
- Skips AMIs that are currently in use by EC2 instances.

Usage:
python ec2_delete_old_amis.py --retention-days DAYS [--dry-run] [--profile PROFILE_NAME]

License: MIT
"""

import argparse
import logging
from datetime import datetime, timedelta, timezone

import boto3
from botocore.exceptions import ClientError


def setup_logging():
    """Sets up basic logging."""
    logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
    return logging.getLogger(__name__)


def get_ec2_client():
    """Creates and returns an EC2 client."""
    try:
        return boto3.client("ec2")
    except ClientError as e:
        logger.error(f"Failed to create EC2 client: {e}")
        raise


def get_owned_amis(ec2_client):
    """Retrieves all AMIs owned by the account."""
    try:
        images = ec2_client.describe_images(Owners=["self"])["Images"]
        logger.info(f"Found {len(images)} owned AMIs.")
        return images
    except ClientError as e:
        logger.error(f"Failed to retrieve owned AMIs: {e}")
        return []


def get_in_use_amis(ec2_client):
    """Retrieves a set of AMI IDs currently used by EC2 instances."""
    try:
        in_use_amis = set()
        paginator = ec2_client.get_paginator("describe_instances")
        # Check all instances regardless of state
        for page in paginator.paginate():
            for reservation in page["Reservations"]:
                for instance in reservation["Instances"]:
                    in_use_amis.add(instance["ImageId"])
        logger.info(f"Found {len(in_use_amis)} AMIs in use by EC2 instances.")
        return in_use_amis
    except ClientError as e:
        logger.error(f"Failed to retrieve in-use AMIs: {e}")
        return set()


def deregister_ami(ec2_client, ami, dry_run=False):
    """Deregisters an AMI and its associated snapshots."""
    image_id = ami["ImageId"]
    image_name = ami.get("Name", "N/A")
    action = "Would deregister" if dry_run else "Deregistering"
    logger.info(f"{action} AMI: {image_id} ({image_name}) and associated snapshots.")
    try:
        ec2_client.deregister_image(ImageId=image_id, DeleteAssociatedSnapshots=True, DryRun=dry_run)
        if not dry_run:
            logger.info(f"Successfully deregistered AMI: {image_id} ({image_name})")
        return True
    except ClientError as e:
        if e.response["Error"]["Code"] == "DryRunOperation":
            # This is the expected outcome for a successful dry run.
            logger.info(f"Dry run successful for AMI: {image_id} ({image_name})")
            return True
        logger.error(f"Failed to deregister AMI {image_id} ({image_name}): {e}")
        return False


def main(dry_run=False, retention_days=None):
    """Main function to orchestrate AMI cleanup."""
    if retention_days is None:
        logger.error("Retention days must be provided.")
        return

    ec2_client = get_ec2_client()
    owned_amis = get_owned_amis(ec2_client)
    in_use_amis = get_in_use_amis(ec2_client)

    if not owned_amis:
        logger.info("No owned AMIs found.")
        return

    cutoff_date = datetime.now(timezone.utc) - timedelta(days=retention_days)

    amis_to_delete = []
    for ami in owned_amis:
        creation_date_str = ami["CreationDate"]
        # Parse ISO 8601 timestamp from AWS
        if creation_date_str.endswith("Z"):
            creation_date_str = creation_date_str[:-1] + "+00:00"
        creation_date = datetime.fromisoformat(creation_date_str)

        if creation_date < cutoff_date:
            if ami["ImageId"] in in_use_amis:
                logger.warning(f"Skipping in-use AMI: {ami['ImageId']} ({ami.get('Name', 'N/A')})")
                continue
            amis_to_delete.append(ami)

    logger.info(f"Found {len(amis_to_delete)} AMIs older than {retention_days} days to be deregistered.")

    if not amis_to_delete:
        logger.info("No old AMIs to deregister.")
        return

    deregistered_count = 0
    for ami in amis_to_delete:
        if deregister_ami(ec2_client, ami, dry_run):
            deregistered_count += 1

    logger.info("--- Cleanup Summary ---")
    logger.info(f"Total owned AMIs: {len(owned_amis)}")
    logger.info(f"AMIs in use: {len(in_use_amis)}")
    logger.info(f"AMIs older than {retention_days} days: {len(amis_to_delete)}")

    if dry_run:
        logger.info(f"Dry run: Would have deregistered {deregistered_count} AMIs.")
        logger.info(f"AMI IDs that would be deregistered: {[ami['ImageId'] for ami in amis_to_delete]}")
    else:
        logger.info(f"Deregistered {deregistered_count} AMIs.")


if __name__ == "__main__":
    logger = setup_logging()

    parser = argparse.ArgumentParser(description="Deregister old AMIs and their snapshots.")
    parser.add_argument(
        "--retention-days", type=int, required=True, help="Number of days to retain AMIs before deregistration."
    )
    parser.add_argument("--dry-run", action="store_true", help="Perform a dry run without actually deregistering AMIs.")
    parser.add_argument("--profile", help="AWS CLI profile name to use.")
    args = parser.parse_args()

    if args.profile:
        boto3.setup_default_session(profile_name=args.profile)

    main(dry_run=args.dry_run, retention_days=args.retention_days)
