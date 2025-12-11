"""
Description: This script searches for CloudWatch Log Groups containing a specified string in their name
and optionally deletes them. It supports a dry-run mode for safe execution.

Key features:
- Supports dry run mode for safe execution
- Provides detailed logging of all operations
- Shows total storage used by matching log groups
- Implements error handling for robustness

Usage:
python cw_delete_log_groups_by_name.py <search-string> [--dry-run]

Author: Danny Steenman
License: MIT
"""

import argparse
import logging
import sys

import boto3
from botocore.exceptions import ClientError


def setup_logging():
    logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
    return logging.getLogger(__name__)


def get_cloudwatch_client():
    try:
        return boto3.client("logs")
    except ClientError as e:
        logger.error(f"Failed to create CloudWatch Logs client: {e}")
        sys.exit(1)


def get_all_log_groups(logs_client):
    """Retrieve all log groups using pagination."""
    log_groups = []
    paginator = logs_client.get_paginator("describe_log_groups")

    try:
        for page in paginator.paginate():
            log_groups.extend(page.get("logGroups", []))
    except ClientError as e:
        logger.error(f"Failed to list log groups: {e}")
        sys.exit(1)

    return log_groups


def delete_log_group(logs_client, log_group_name, dry_run=False):
    """Delete a single log group."""
    try:
        if dry_run:
            logger.info(f"Would delete log group: {log_group_name}")
        else:
            logs_client.delete_log_group(logGroupName=log_group_name)
            logger.info(f"Deleted log group: {log_group_name}")
        return True
    except ClientError as e:
        logger.error(f"Failed to delete log group {log_group_name}: {e}")
        return False


def bytes_to_human_readable(size_bytes):
    """Convert bytes to human readable format."""
    if size_bytes == 0:
        return "0 B"
    for unit in ["B", "KB", "MB", "GB", "TB"]:
        if abs(size_bytes) < 1024.0:
            return f"{size_bytes:.2f} {unit}"
        size_bytes /= 1024.0
    return f"{size_bytes:.2f} PB"


def main(search_string, dry_run=False):
    logs_client = get_cloudwatch_client()

    logger.info(f"Searching for log groups containing: '{search_string}'")
    all_log_groups = get_all_log_groups(logs_client)

    # Filter log groups by search string
    matching_log_groups = [lg for lg in all_log_groups if search_string in lg["logGroupName"]]

    if not matching_log_groups:
        logger.info(f"No log groups found containing the string: '{search_string}'")
        return

    logger.info(f"Found {len(matching_log_groups)} log group(s) matching '{search_string}'")

    total_storage = 0
    deleted_count = 0
    failed_count = 0

    for log_group in matching_log_groups:
        log_group_name = log_group["logGroupName"]
        stored_bytes = log_group.get("storedBytes", 0)
        total_storage += stored_bytes

        logger.info(f"Found log group: {log_group_name} (Size: {bytes_to_human_readable(stored_bytes)})")

        if delete_log_group(logs_client, log_group_name, dry_run):
            deleted_count += 1
        else:
            failed_count += 1

    logger.info(f"Total storage used by matching log groups: {bytes_to_human_readable(total_storage)}")

    if dry_run:
        logger.info(f"Dry run completed. Would have deleted {deleted_count} log group(s).")
    else:
        logger.info(f"Operation completed. Deleted {deleted_count} log group(s), {failed_count} failed.")


if __name__ == "__main__":
    logger = setup_logging()

    parser = argparse.ArgumentParser(description="Delete CloudWatch Log Groups containing a specific string")
    parser.add_argument("search_string", help="String to search for in log group names")
    parser.add_argument("--dry-run", action="store_true", help="Perform a dry run without actually deleting anything")
    args = parser.parse_args()

    main(args.search_string, args.dry_run)
