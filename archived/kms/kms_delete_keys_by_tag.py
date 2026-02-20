#!/usr/bin/env python3
#  https://github.com/towardsthecloud/aws-toolbox
#
#  License: MIT
#
# This script schedules deletion for AWS KMS keys based on tag filters.
#
# It identifies customer-managed KMS keys by tag key-value pairs and schedules
# them for deletion with a configurable waiting period. You can exclude specific
# key IDs from deletion using a comma-separated list.
#
# Features:
#   - Filter keys by tag key and value
#   - Exclude specific key IDs from deletion
#   - Shows key aliases, descriptions, state, and tags
#   - Interactive confirmation before scheduling deletion
#   - Dry-run mode to preview actions without making changes
#   - Configurable deletion waiting period (7-30 days)
#
# Input:
#   - --tag-key: Tag key to filter on (required)
#   - --tag-value: Tag value to filter on (required)
#   - --exclude-keys: Comma-separated list of key IDs to exclude (optional)
#   - --region: AWS region to search (optional, uses default AWS config if omitted)
#   - --profile: AWS profile name (optional)
#   - --pending-days: Days to wait before deletion (7-30, default: 7)
#   - --dry-run: Preview actions without scheduling deletion
#   - --no-confirm: Skip confirmation prompts (use with caution)
#
# Examples:
#   # Delete keys with specific tag, excluding certain key IDs
#   python kms_delete_keys_by_tag.py --tag-key repo-name --tag-value my-project-artifacts \
#     --exclude-keys 12345678-1234-1234-1234-123456789012,abcdef12-abcd-abcd-abcd-abcdef123456
#
#   # Dry-run to preview which keys would be affected
#   python kms_delete_keys_by_tag.py --tag-key Environment --tag-value test --dry-run
#
#   # Delete with 30-day waiting period
#   python kms_delete_keys_by_tag.py --tag-key project --tag-value old-app --pending-days 30
#
#   # Use specific region and profile
#   python kms_delete_keys_by_tag.py --tag-key team --tag-value legacy \
#     --region us-west-2 --profile production
#
# Note: Keys scheduled for deletion can be cancelled during the waiting period
#       using: aws kms cancel-key-deletion --key-id <key-id>
#
# Requirements:
#   - Requires permissions: kms:ListKeys, kms:DescribeKey, kms:ListAliases,
#     kms:ListResourceTags, kms:ScheduleKeyDeletion

import argparse
import sys

import boto3
from botocore.exceptions import ClientError


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Schedule deletion for AWS KMS keys based on tags")
    parser.add_argument(
        "--tag-key",
        required=True,
        help="Tag key to filter on (required)",
    )
    parser.add_argument(
        "--tag-value",
        required=True,
        help="Tag value to filter on (required)",
    )
    parser.add_argument(
        "--exclude-keys",
        help="Comma-separated list of key IDs to exclude from deletion",
    )
    parser.add_argument(
        "--region",
        help="AWS region to use (falls back to default boto3 resolution if omitted)",
    )
    parser.add_argument(
        "--profile",
        help="AWS profile name to use from your credentials file",
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


def get_key_tags(kms_client, key_id: str) -> dict[str, str]:
    """Get all tags for a given KMS key"""
    try:
        response = kms_client.list_resource_tags(KeyId=key_id)
        tags = {}
        for tag in response.get("Tags", []):
            tags[tag["TagKey"]] = tag["TagValue"]
        return tags
    except ClientError:
        return {}


def list_keys_by_tag(kms_client, tag_key: str, tag_value: str, exclude_key_ids: set[str]):
    """List all customer-managed KMS keys matching the tag filter"""
    keys_info = []

    try:
        paginator = kms_client.get_paginator("list_keys")
        processed_keys = 0

        print("  Scanning KMS keys for matching tags...", file=sys.stderr)

        for page in paginator.paginate():
            for key in page.get("Keys", []):
                key_id = key["KeyId"]

                # Skip excluded keys
                if key_id in exclude_key_ids:
                    continue

                try:
                    # Get detailed key metadata
                    metadata = kms_client.describe_key(KeyId=key_id)
                    key_metadata = metadata["KeyMetadata"]

                    # Skip AWS-managed keys
                    if key_metadata.get("KeyManager") == "AWS":
                        continue

                    # Skip keys pending deletion
                    key_state = key_metadata.get("KeyState", "Unknown")
                    if key_state == "PendingDeletion":
                        continue

                    processed_keys += 1
                    print(f"  Processed {processed_keys} customer-managed keys...", end="\r", file=sys.stderr)

                    # Get tags for this key
                    tags = get_key_tags(kms_client, key_id)

                    # Check if tag matches
                    if tags.get(tag_key) != tag_value:
                        continue

                    # Get key aliases
                    aliases = get_key_aliases(kms_client, key_id)

                    keys_info.append(
                        {
                            "KeyId": key_id,
                            "Arn": key_metadata.get("Arn", "N/A"),
                            "Description": key_metadata.get("Description", "No description"),
                            "State": key_state,
                            "CreationDate": key_metadata.get("CreationDate"),
                            "Aliases": aliases if aliases else ["(no alias)"],
                            "Tags": tags,
                        }
                    )

                except ClientError as error:
                    error_code = error.response.get("Error", {}).get("Code", "")
                    # Skip keys we can't access
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


def display_keys(keys_info: list, tag_key: str, tag_value: str):
    """Display KMS keys in a formatted table"""
    if not keys_info:
        print(f"\nNo customer-managed KMS keys found with tag {tag_key}={tag_value}")
        return

    print(f"\nFound {len(keys_info)} customer-managed KMS key(s) with tag {tag_key}={tag_value}:\n")
    print("=" * 140)

    for idx, key in enumerate(keys_info, 1):
        aliases_str = ", ".join(key["Aliases"])
        creation_date = key["CreationDate"].strftime("%Y-%m-%d") if key["CreationDate"] else "N/A"

        print(f"{idx}. Key ID: {key['KeyId']}")
        print(f"   Aliases: {aliases_str}")
        print(f"   Description: {key['Description']}")
        print(f"   State: {key['State']}")
        print(f"   Created: {creation_date}")

        # Display all tags
        if key["Tags"]:
            tags_str = ", ".join([f"{k}={v}" for k, v in key["Tags"].items()])
            print(f"   Tags: {tags_str}")

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

    # Get current region for display
    region = session.region_name or "default region"

    # Parse excluded key IDs
    exclude_key_ids = set()
    if args.exclude_keys:
        exclude_key_ids = {key_id.strip() for key_id in args.exclude_keys.split(",")}
        print(f"Excluding {len(exclude_key_ids)} key(s) from deletion")

    print(f"Scanning for customer-managed KMS keys in {region}...")
    print(f"Filter: tag {args.tag_key}={args.tag_value}\n")

    # List all keys matching the tag filter
    keys_info = list_keys_by_tag(kms_client, args.tag_key, args.tag_value, exclude_key_ids)

    # Display keys
    display_keys(keys_info, args.tag_key, args.tag_value)

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
    print("=" * 140)
    print(f"Summary: {success_count} succeeded, {failed_count} failed")

    if args.dry_run:
        print("\n[DRY-RUN] No actual changes were made.")
    elif success_count > 0:
        print(f"\nKeys can be recovered within {args.pending_days} days using:")
        print("  aws kms cancel-key-deletion --key-id <key-id>")

    return 0 if failed_count == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
