#!/usr/bin/env python3
#  https://github.com/towardsthecloud/aws-toolbox
#
#  License: MIT
#
# This script schedules deletion for unused AWS KMS keys.
#
# It identifies customer-managed KMS keys that are candidates for deletion by:
# - Checking CloudTrail for recent cryptographic operations (Encrypt, Decrypt, etc.)
# - Filtering by key state (enabled/disabled)
# - Manual selection through interactive prompts
#
# The script schedules keys for deletion with a 7-day waiting period by default.
# During this period, keys can be recovered if needed. AWS-managed keys are automatically
# excluded from the search as they cannot be deleted.
#
# Features:
#   - Queries CloudTrail to determine last usage of each key
#   - Shows key aliases, descriptions, state, and last used date
#   - Filters keys based on days since last use
#   - Allows filtering by key state (enabled/disabled)
#   - Interactive confirmation before scheduling deletion
#   - Dry-run mode to preview actions without making changes
#   - Configurable deletion waiting period (7-30 days)
#   - Works with or without CloudTrail (graceful degradation)
#
# Input:
#   - --region: AWS region to search (optional, uses default AWS config if omitted)
#   - --profile: AWS profile name (optional)
#   - --unused-days: Only show keys unused for this many days (default: 90)
#   - --check-usage-days: How far back to check CloudTrail (default: 90, max: 90)
#   - --pending-days: Days to wait before deletion (7-30, default: 7)
#   - --state: Filter by key state: enabled, disabled, or all (default: all)
#   - --skip-usage-check: Skip CloudTrail usage check (faster but less accurate)
#   - --dry-run: Preview actions without scheduling deletion
#   - --no-confirm: Skip confirmation prompts (use with caution)
#
# Examples:
#   # Find keys unused for 90+ days and check their usage
#   python kms_delete_unused_keys.py
#
#   # Find keys unused for 180 days, check last 90 days of usage
#   python kms_delete_unused_keys.py --unused-days 180
#
#   # Schedule deletion for disabled keys with 30-day waiting period
#   python kms_delete_unused_keys.py --state disabled --pending-days 30
#
#   # Dry-run to preview which keys would be affected
#   python kms_delete_unused_keys.py --dry-run
#
#   # Skip CloudTrail check (faster, but only lists all keys)
#   python kms_delete_unused_keys.py --skip-usage-check
#
#   # Use specific region and profile
#   python kms_delete_unused_keys.py --region us-west-2 --profile production
#
# Note: Keys scheduled for deletion can be cancelled during the waiting period
#       using: aws kms cancel-key-deletion --key-id <key-id>
#
# Requirements:
#   - CloudTrail must be enabled for accurate usage tracking
#   - Requires permissions: kms:ListKeys, kms:DescribeKey, kms:ListAliases,
#     cloudtrail:LookupEvents, kms:ScheduleKeyDeletion

import argparse
import sys
from datetime import datetime, timedelta, timezone

import boto3
from botocore.exceptions import ClientError


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Schedule deletion for unused AWS KMS customer-managed keys")
    parser.add_argument(
        "--region",
        help="AWS region to use (falls back to default boto3 resolution if omitted)",
    )
    parser.add_argument(
        "--profile",
        help="AWS profile name to use from your credentials file",
    )
    parser.add_argument(
        "--unused-days",
        type=int,
        default=90,
        help="Only show keys unused for this many days (default: 90)",
    )
    parser.add_argument(
        "--check-usage-days",
        type=int,
        default=90,
        help="How far back to check CloudTrail for usage (default: 90, max: 90)",
    )
    parser.add_argument(
        "--pending-days",
        type=int,
        default=7,
        choices=range(7, 31),
        metavar="[7-30]",
        help="Number of days before deletion (7-30, default: 7)",
    )
    parser.add_argument(
        "--state",
        choices=["enabled", "disabled", "all"],
        default="all",
        help="Filter keys by state (default: all)",
    )
    parser.add_argument(
        "--skip-usage-check",
        action="store_true",
        help="Skip CloudTrail usage check (faster but lists all keys)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Preview actions without scheduling deletion",
    )
    parser.add_argument(
        "--no-confirm",
        action="store_true",
        help="Skip confirmation prompts (use with caution)",
    )
    return parser.parse_args()


def build_session(region: str | None, profile: str | None) -> boto3.session.Session:
    session_kwargs = {}
    if profile:
        session_kwargs["profile_name"] = profile
    if region:
        session_kwargs["region_name"] = region
    return boto3.session.Session(**session_kwargs)


def get_key_aliases(kms_client, key_id: str) -> list[str]:
    """Get all aliases for a given KMS key"""
    try:
        paginator = kms_client.get_paginator("list_aliases")
        aliases = []
        for page in paginator.paginate(KeyId=key_id):
            for alias in page.get("Aliases", []):
                if "AliasName" in alias:
                    aliases.append(alias["AliasName"])
        return aliases
    except ClientError:
        return []


def build_key_usage_map(cloudtrail_client, check_days: int) -> dict[str, datetime]:
    """
    Build a map of all KMS key usage from CloudTrail.
    Returns a dictionary mapping key_id -> last_used_datetime.
    This is much more efficient than querying per key.
    """
    # KMS events that indicate actual key usage
    kms_usage_events = [
        "Encrypt",
        "Decrypt",
        "GenerateDataKey",
        "GenerateDataKeyWithoutPlaintext",
        "ReEncrypt",
    ]

    start_time = datetime.now(timezone.utc) - timedelta(days=check_days)
    key_usage_map = {}

    print("  Fetching KMS usage events from CloudTrail...", file=sys.stderr)

    try:
        # Query all KMS events at once
        paginator = cloudtrail_client.get_paginator("lookup_events")

        for page in paginator.paginate(
            LookupAttributes=[
                {"AttributeKey": "ResourceType", "AttributeValue": "AWS::KMS::Key"}
            ],
            StartTime=start_time,
        ):
            events = page.get("Events", [])

            for event in events:
                event_name = event.get("EventName", "")

                # Only process cryptographic operations
                if event_name not in kms_usage_events:
                    continue

                event_time = event.get("EventTime")
                if not event_time:
                    continue

                # Extract key IDs from the event
                # CloudTrail stores key info in Resources
                resources = event.get("Resources", [])
                for resource in resources:
                    if resource.get("ResourceType") == "AWS::KMS::Key":
                        resource_name = resource.get("ResourceName")
                        if resource_name:
                            # Resource name could be ARN or key ID, extract key ID
                            key_id = resource_name.split("/")[-1] if "/" in resource_name else resource_name

                            # Update last used time for this key
                            if key_id not in key_usage_map or event_time > key_usage_map[key_id]:
                                key_usage_map[key_id] = event_time

        print(f"  Found usage data for {len(key_usage_map)} key(s) in CloudTrail", file=sys.stderr)

    except ClientError as error:
        error_code = error.response.get("Error", {}).get("Code", "")
        # Handle case where CloudTrail is not available or not enabled
        if error_code in ["TrailNotFoundException", "AccessDeniedException"]:
            print("  Warning: CloudTrail not accessible, skipping usage check", file=sys.stderr)
            return {}
        print(f"  Warning: Error querying CloudTrail: {error}", file=sys.stderr)

    return key_usage_map


def list_customer_managed_keys(kms_client, key_usage_map: dict[str, datetime], state_filter: str, unused_days: int, skip_usage_check: bool):
    """List all customer-managed KMS keys with usage information"""
    keys_info = []

    try:
        paginator = kms_client.get_paginator("list_keys")
        total_keys = 0
        processed_keys = 0

        print("  Processing KMS keys...", file=sys.stderr)

        for page in paginator.paginate():
            for key in page.get("Keys", []):
                key_id = key["KeyId"]
                total_keys += 1

                try:
                    # Get detailed key metadata
                    metadata = kms_client.describe_key(KeyId=key_id)
                    key_metadata = metadata["KeyMetadata"]

                    # Skip AWS-managed keys
                    if key_metadata.get("KeyManager") == "AWS":
                        continue

                    processed_keys += 1
                    print(f"  Processed {processed_keys} customer-managed keys...", end="\r", file=sys.stderr)

                    key_state = key_metadata.get("KeyState", "Unknown")

                    # Filter by state if specified
                    if state_filter != "all":
                        if state_filter == "enabled" and key_state != "Enabled":
                            continue
                        if state_filter == "disabled" and key_state != "Disabled":
                            continue

                    # Skip keys pending deletion
                    if key_state == "PendingDeletion":
                        continue

                    # Get key aliases
                    aliases = get_key_aliases(kms_client, key_id)

                    # Check usage from the pre-built map
                    last_used = None
                    days_since_last_use = None

                    if not skip_usage_check:
                        last_used = key_usage_map.get(key_id)

                        if last_used:
                            days_since_last_use = (datetime.now(timezone.utc) - last_used).days
                            # Skip keys that have been used recently
                            if days_since_last_use < unused_days:
                                continue
                        else:
                            # Key was never used in the check period
                            creation_date = key_metadata.get("CreationDate")
                            if creation_date:
                                days_since_creation = (datetime.now(timezone.utc) - creation_date).days
                                # If created recently, don't consider it unused
                                if days_since_creation < unused_days:
                                    continue
                                # Set days_since_last_use to days since creation (capped at check period)
                                days_since_last_use = days_since_creation

                    keys_info.append(
                        {
                            "KeyId": key_id,
                            "Arn": key_metadata.get("Arn", "N/A"),
                            "Description": key_metadata.get("Description", "No description"),
                            "State": key_state,
                            "CreationDate": key_metadata.get("CreationDate"),
                            "Aliases": aliases if aliases else ["(no alias)"],
                            "LastUsed": last_used,
                            "DaysSinceLastUse": days_since_last_use,
                        }
                    )

                except ClientError as error:
                    error_code = error.response.get("Error", {}).get("Code", "")
                    # Skip keys we can't access (common in some AWS accounts)
                    if error_code == "AccessDeniedException":
                        continue
                    print(
                        f"\n  Warning: Unable to get details for key {key_id}: {error}",
                        file=sys.stderr,
                    )
                    continue

        print(" " * 80, end="\r", file=sys.stderr)  # Clear the progress line

    except ClientError as error:
        print(f"Error listing KMS keys: {error}", file=sys.stderr)
        sys.exit(1)

    return keys_info


def display_keys(keys_info: list, skip_usage_check: bool):
    """Display KMS keys in a formatted table"""
    if not keys_info:
        print("\nNo customer-managed KMS keys found matching the criteria.")
        return

    print(f"\nFound {len(keys_info)} unused customer-managed KMS key(s):\n")
    print("=" * 140)

    for idx, key in enumerate(keys_info, 1):
        aliases_str = ", ".join(key["Aliases"])
        creation_date = key["CreationDate"].strftime("%Y-%m-%d") if key["CreationDate"] else "N/A"

        print(f"{idx}. Key ID: {key['KeyId']}")
        print(f"   Aliases: {aliases_str}")
        print(f"   Description: {key['Description']}")
        print(f"   State: {key['State']}")
        print(f"   Created: {creation_date}")

        if not skip_usage_check:
            last_used = key.get("LastUsed")
            days_since_last_use = key.get("DaysSinceLastUse")

            if last_used:
                last_used_str = last_used.strftime("%Y-%m-%d %H:%M:%S UTC")
                print(f"   Last Used: {last_used_str} ({days_since_last_use} days ago)")
            else:
                print(f"   Last Used: Never (in last {days_since_last_use}+ days)")

        print("-" * 140)


def confirm_deletion(keys_info: list, pending_days: int, no_confirm: bool) -> bool:
    """Ask user to confirm deletion"""
    if no_confirm:
        return True

    print(f"\n⚠️  WARNING: This will schedule {len(keys_info)} key(s) for deletion!")
    print(f"   Waiting period: {pending_days} days")
    print("   Keys can be recovered during this period using: aws kms cancel-key-deletion")

    response = input("\nDo you want to proceed? (yes/no): ").strip().lower()
    return response == "yes"


def schedule_key_deletion(kms_client, key_id: str, pending_days: int, dry_run: bool) -> tuple[bool, str]:
    """Schedule a KMS key for deletion"""
    if dry_run:
        return True, f"[DRY-RUN] Would schedule key {key_id} for deletion in {pending_days} days"

    try:
        response = kms_client.schedule_key_deletion(KeyId=key_id, PendingWindowInDays=pending_days)
        deletion_date = response.get("DeletionDate")
        deletion_date_str = deletion_date.strftime("%Y-%m-%d %H:%M:%S") if deletion_date else "N/A"
        return True, f"✓ Scheduled for deletion on {deletion_date_str}"
    except ClientError as error:
        return False, f"✗ Failed: {error}"


def main() -> int:
    args = parse_args()
    session = build_session(args.region, args.profile)
    kms_client = session.client("kms")
    cloudtrail_client = session.client("cloudtrail")

    # Get current region for display
    region = session.region_name or "default region"

    print(f"Scanning for customer-managed KMS keys in {region}...")
    print(f"Filter: {args.state} keys")

    # Build key usage map from CloudTrail (if not skipped)
    key_usage_map = {}
    if not args.skip_usage_check:
        print(f"Checking CloudTrail for keys unused in the last {args.unused_days} days...")
        print(f"(Looking back {args.check_usage_days} days in CloudTrail)\n")
        key_usage_map = build_key_usage_map(cloudtrail_client, args.check_usage_days)
        print()
    else:
        print("Skipping CloudTrail usage check (--skip-usage-check enabled)\n")

    # List all customer-managed keys
    keys_info = list_customer_managed_keys(
        kms_client,
        key_usage_map,
        args.state,
        args.unused_days,
        args.skip_usage_check
    )

    # Display keys
    display_keys(keys_info, args.skip_usage_check)

    if not keys_info:
        return 0

    # Confirm deletion
    if not confirm_deletion(keys_info, args.pending_days, args.no_confirm):
        print("\nOperation cancelled.")
        return 0

    # Schedule deletion for each key
    print(f"\n{'[DRY-RUN] ' if args.dry_run else ''}Scheduling key deletion...\n")

    success_count = 0
    failed_count = 0

    for key in keys_info:
        key_id = key["KeyId"]
        aliases_str = ", ".join(key["Aliases"])

        success, message = schedule_key_deletion(kms_client, key_id, args.pending_days, args.dry_run)

        print(f"Key: {key_id} ({aliases_str})")
        print(f"  {message}")
        print()

        if success:
            success_count += 1
        else:
            failed_count += 1

    # Summary
    print("=" * 120)
    print(f"Summary: {success_count} succeeded, {failed_count} failed")

    if args.dry_run:
        print("\n[DRY-RUN] No actual changes were made.")
    elif success_count > 0:
        print(f"\nKeys can be recovered within {args.pending_days} days using:")
        print("  aws kms cancel-key-deletion --key-id <key-id>")

    return 0 if failed_count == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
