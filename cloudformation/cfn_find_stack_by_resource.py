#!/usr/bin/env python3
#  https://github.com/towardsthecloud/aws-toolbox
#
#  License: MIT
#
# This script locates the CloudFormation stack that owns a given resource.
#
# It searches through all CloudFormation stacks in your account (excluding deleted stacks)
# and finds which stack contains a resource matching your search criteria. The search
# compares against both the logical resource ID and physical resource ID.
#
# By default, it performs a case-insensitive substring match, but you can use --exact
# for precise matching. Use --include-nested to search within nested CloudFormation stacks.
#
# Input:
#   - RESOURCE: The resource identifier to search for (required)
#   - --region: AWS region (optional, uses default AWS config if omitted)
#   - --profile: AWS profile name (optional)
#   - --exact: Perform exact case-sensitive match instead of substring search
#   - --include-nested: Include nested stacks in the search
#
# Examples:
#   # Find stack containing a Lambda function (substring match)
#   python cfn_find_stack_by_resource.py MyLambdaFunction
#
#   # Find stack with exact resource ID match
#   python cfn_find_stack_by_resource.py --exact MyExactResourceId
#
#   # Search in specific region and profile
#   python cfn_find_stack_by_resource.py MyBucket --region us-west-2 --profile production
#
#   # Search including nested stacks
#   python cfn_find_stack_by_resource.py MyResource --include-nested

import argparse
import sys
from collections import defaultdict

import boto3
from botocore.exceptions import ClientError


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Find the CloudFormation stack that contains a resource with a "
            "matching logical or physical resource ID."
        )
    )
    parser.add_argument(
        "resource_name",
        metavar="RESOURCE",
        help="Resource identifier to search for (case-insensitive substring match)",
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
        "--exact",
        action="store_true",
        help="Require an exact (case-sensitive) match instead of a substring lookup",
    )
    parser.add_argument(
        "--include-nested",
        action="store_true",
        help="Inspect nested stacks referenced by parent stacks (costs extra API calls)",
    )
    return parser.parse_args()


def build_session(region: str | None, profile: str | None) -> boto3.session.Session:
    session_kwargs = {}
    if profile:
        session_kwargs["profile_name"] = profile
    if region:
        session_kwargs["region_name"] = region
    return boto3.session.Session(**session_kwargs)


def matches(target: str | None, needle: str, exact: bool) -> bool:
    if target is None:
        return False
    if exact:
        return target == needle
    return needle.lower() in target.lower()


def stack_resource_iterator(cf_client, stack_name):
    paginator = cf_client.get_paginator("list_stack_resources")
    for page in paginator.paginate(StackName=stack_name):
        for resource in page.get("StackResourceSummaries", []):
            yield resource


def find_matching_stacks(cf_client, resource_name: str, exact: bool, include_nested: bool):
    matches_by_stack = defaultdict(list)
    paginator = cf_client.get_paginator("list_stacks")

    for page in paginator.paginate():
        for summary in page.get("StackSummaries", []):
            status = summary.get("StackStatus", "")
            if status == "DELETE_COMPLETE":
                continue

            stack_id = summary.get("StackId")
            stack_name = summary.get("StackName")

            try:
                for resource in stack_resource_iterator(cf_client, stack_id):
                    logical_id = resource.get("LogicalResourceId")
                    physical_id = resource.get("PhysicalResourceId")

                    if matches(logical_id, resource_name, exact) or matches(
                        physical_id, resource_name, exact
                    ):
                        matches_by_stack[stack_id].append(resource)

                    if (
                        include_nested
                        and resource.get("ResourceType") == "AWS::CloudFormation::Stack"
                        and resource.get("PhysicalResourceId")
                    ):
                        nested_stack_id = resource["PhysicalResourceId"]
                        try:
                            for nested_resource in stack_resource_iterator(
                                cf_client, nested_stack_id
                            ):
                                logical_nested = nested_resource.get("LogicalResourceId")
                                physical_nested = nested_resource.get("PhysicalResourceId")
                                if matches(
                                    logical_nested, resource_name, exact
                                ) or matches(physical_nested, resource_name, exact):
                                    matches_by_stack[nested_stack_id].append(
                                        nested_resource
                                    )
                        except ClientError as nested_error:
                            print(
                                f"Warning: unable to inspect nested stack {nested_stack_id}: {nested_error}",
                                file=sys.stderr,
                            )

            except ClientError as error:
                print(
                    f"Warning: unable to inspect stack {stack_name} ({stack_id}): {error}",
                    file=sys.stderr,
                )
                continue

    return matches_by_stack


def display_results(matches_by_stack):
    if not matches_by_stack:
        print("No matching resources found in your CloudFormation stacks.")
        return 1

    for stack_id, resources in matches_by_stack.items():
        print(f"Stack: {stack_id}")
        for resource in resources:
            logical_id = resource.get("LogicalResourceId", "-")
            physical_id = resource.get("PhysicalResourceId", "-")
            resource_type = resource.get("ResourceType", "-")
            print(
                f"  - LogicalId: {logical_id}, PhysicalId: {physical_id}, Type: {resource_type}"
            )
        print()

    return 0


def main() -> int:
    args = parse_args()
    session = build_session(args.region, args.profile)
    cf_client = session.client("cloudformation")

    matches_by_stack = find_matching_stacks(
        cf_client,
        resource_name=args.resource_name,
        exact=args.exact,
        include_nested=args.include_nested,
    )

    return display_results(matches_by_stack)


if __name__ == "__main__":
    raise SystemExit(main())
