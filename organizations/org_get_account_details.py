#  https://github.com/towardsthecloud/aws-toolbox
#
#  License: MIT
#
# This script finds and displays detailed information about an AWS account by account ID.
#
# It retrieves comprehensive details about a specific AWS account within your organization,
# including account metadata, organizational placement (OU path), and associated tags.
# The script validates that the account exists and is part of your organization.
#
# Information displayed:
#   - Account name and ID
#   - Email address associated with the account
#   - Account status (e.g., ACTIVE, SUSPENDED)
#   - Account ARN
#   - How the account joined (INVITED or CREATED)
#   - When the account joined the organization
#   - Full organizational unit (OU) path
#   - All tags associated with the account
#
# Input:
#   - ACCOUNT_ID: 12-digit AWS account ID (required)
#     Can be provided as command-line argument or will be prompted
#
# Examples:
#   # Find account with ID provided as argument
#   python org_find_account.py 123456789012
#
#   # Run without argument (will prompt for account ID)
#   python org_find_account.py
#
#   # Use with specific AWS profile
#   AWS_PROFILE=production python org_find_account.py 123456789012

import sys

import boto3
from botocore.exceptions import ClientError


def get_account_details(account_id, organizations_client):
    """Get detailed information about an account"""
    try:
        response = organizations_client.describe_account(AccountId=account_id)
        return response["Account"]
    except ClientError as e:
        if e.response["Error"]["Code"] == "AccountNotFoundException":
            return None
        raise


def get_parent_ou_path(account_id, organizations_client):
    """Get the full OU path for an account"""
    try:
        # Get immediate parent
        parents = organizations_client.list_parents(ChildId=account_id)
        if not parents["Parents"]:
            return "/"

        parent = parents["Parents"][0]
        parent_id = parent["Id"]
        parent_type = parent["Type"]

        # If parent is root, return root
        if parent_type == "ROOT":
            roots = organizations_client.list_roots()
            root_name = roots["Roots"][0].get("Name", "Root")
            return f"/{root_name}"

        # Build path recursively
        path_parts = []
        current_id = parent_id

        while True:
            try:
                # Get OU details
                ou_response = organizations_client.describe_organizational_unit(
                    OrganizationalUnitId=current_id
                )
                ou_name = ou_response["OrganizationalUnit"]["Name"]
                path_parts.insert(0, ou_name)

                # Get parent of this OU
                parents = organizations_client.list_parents(ChildId=current_id)
                if not parents["Parents"]:
                    break

                parent = parents["Parents"][0]
                parent_type = parent["Type"]

                if parent_type == "ROOT":
                    roots = organizations_client.list_roots()
                    root_name = roots["Roots"][0].get("Name", "Root")
                    path_parts.insert(0, root_name)
                    break

                current_id = parent["Id"]
            except ClientError:
                break

        return "/" + "/".join(path_parts) if path_parts else "/"

    except ClientError:
        return "Unknown"


def get_tags(account_id, organizations_client):
    """Get tags for an account"""
    try:
        response = organizations_client.list_tags_for_resource(ResourceId=account_id)
        return response.get("Tags", [])
    except ClientError:
        return []


def format_account_details(account, parent_path, tags):
    """Format account details for display"""
    output = []
    output.append("\n" + "=" * 70)
    output.append("AWS Account Details")
    output.append("=" * 70)
    output.append("")
    output.append(f"Account Name:       {account.get('Name', 'N/A')}")
    output.append(f"Account ID:         {account['Id']}")
    output.append(f"Email:              {account.get('Email', 'N/A')}")
    output.append(f"Status:             {account.get('Status', 'N/A')}")
    output.append(f"ARN:                {account.get('Arn', 'N/A')}")
    output.append(f"Joined Method:      {account.get('JoinedMethod', 'N/A')}")
    output.append(f"Joined Date:        {account.get('JoinedTimestamp', 'N/A')}")
    output.append(f"Organization Path:  {parent_path}")

    if tags:
        output.append("")
        output.append("Tags:")
        for tag in tags:
            output.append(f"  {tag['Key']}: {tag['Value']}")
    else:
        output.append("")
        output.append("Tags:               None")

    output.append("")
    output.append("=" * 70)

    return "\n".join(output)


def main():
    """Main function"""
    # Get account ID from command line argument or prompt
    if len(sys.argv) > 1:
        account_id = sys.argv[1]
    else:
        try:
            account_id = input("Enter AWS Account ID: ").strip()
        except (KeyboardInterrupt, EOFError):
            print("\nOperation cancelled.", file=sys.stderr)
            sys.exit(0)

    if not account_id:
        print("Error: Account ID is required", file=sys.stderr)
        sys.exit(1)

    # Validate account ID format (12 digits)
    if not account_id.isdigit() or len(account_id) != 12:
        print("Error: Account ID must be exactly 12 digits", file=sys.stderr)
        sys.exit(1)

    try:
        # Create AWS Organizations client
        organizations = boto3.client("organizations")

        print(f"Searching for account {account_id}...", file=sys.stderr)

        # Get account details
        account = get_account_details(account_id, organizations)

        if not account:
            print(f"\nAccount {account_id} not found in this organization.", file=sys.stderr)
            sys.exit(1)

        # Get parent OU path
        parent_path = get_parent_ou_path(account_id, organizations)

        # Get tags
        tags = get_tags(account_id, organizations)

        # Format and display results
        output = format_account_details(account, parent_path, tags)
        print(output)

    except ClientError as e:
        error_code = e.response["Error"]["Code"]
        error_message = e.response["Error"]["Message"]
        print(f"AWS Error ({error_code}): {error_message}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error: {str(e)}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
