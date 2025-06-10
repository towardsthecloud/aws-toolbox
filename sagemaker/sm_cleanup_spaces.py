#!/usr/bin/env python3
"""
SageMaker Spaces Cleanup Tool

This script lists all active SageMaker Studio spaces and allows you to
interactively select which ones to delete using spacebar selection.

Requirements:
- boto3
- inquirer
- AWS credentials configured

Usage:
    pip install -r requirements.txt
    python sm_cleanup_spaces.py

Author: Danny Steenman
License: MIT
"""

import boto3
import inquirer
import sys
from typing import List, Dict
from botocore.exceptions import ClientError, NoCredentialsError


class SageMakerSpaceManager:
    def __init__(self, region_name: str = None):
        """Initialize the SageMaker client."""
        try:
            self.session = boto3.Session()
            self.sagemaker = self.session.client('sagemaker', region_name=region_name)
            self.region = region_name or self.session.region_name
            print(f"Connected to SageMaker in region: {self.region}")
        except NoCredentialsError:
            print("Error: AWS credentials not found. Please configure your AWS credentials.")
            sys.exit(1)
        except Exception as e:
            print(f"Error initializing SageMaker client: {str(e)}")
            sys.exit(1)

    def list_domains(self) -> List[Dict]:
        """List all SageMaker Studio domains."""
        try:
            paginator = self.sagemaker.get_paginator('list_domains')
            domains = []
            for page in paginator.paginate():
                domains.extend(page.get('Domains', []))
            return domains
        except ClientError as e:
            print(f"Error listing domains: {str(e)}")
            return []

    def list_spaces_in_domain(self, domain_id: str) -> List[Dict]:
        """List all spaces in a specific domain."""
        try:
            paginator = self.sagemaker.get_paginator('list_spaces')
            spaces = []
            for page in paginator.paginate(DomainIdEquals=domain_id):
                spaces.extend(page.get('Spaces', []))
            return spaces
        except ClientError as e:
            print(f"Error listing spaces in domain {domain_id}: {str(e)}")
            return []

    def get_space_details(self, domain_id: str, space_name: str) -> Dict:
        """Get detailed information about a specific space."""
        try:
            response = self.sagemaker.describe_space(
                DomainId=domain_id,
                SpaceName=space_name
            )
            return response
        except ClientError as e:
            print(f"Error getting details for space {space_name}: {str(e)}")
            return {}

    def list_all_active_spaces(self) -> List[Dict]:
        """List all active spaces across all domains."""
        all_spaces = []
        domains = self.list_domains()

        if not domains:
            print("No SageMaker Studio domains found in this region.")
            return []

        print(f"Found {len(domains)} domain(s). Scanning for spaces...")

        for domain in domains:
            domain_id = domain['DomainId']
            domain_name = domain.get('DomainName', 'Unknown')
            print(f"Checking domain: {domain_name} ({domain_id})")

            spaces = self.list_spaces_in_domain(domain_id)

            for space in spaces:
                space_name = space['SpaceName']
                space_status = space.get('Status', 'Unknown')

                # Get additional details
                details = self.get_space_details(domain_id, space_name)

                space_info = {
                    'domain_id': domain_id,
                    'domain_name': domain_name,
                    'space_name': space_name,
                    'status': space_status,
                    'creation_time': space.get('CreationTime', 'Unknown'),
                    'last_modified_time': space.get('LastModifiedTime', 'Unknown'),
                    'space_settings': details.get('SpaceSettings', {}),
                    'display_name': f"{space_name} (Domain: {domain_name}, Status: {space_status})"
                }

                # Only include spaces that are not already being deleted
                if space_status not in ['Deleting', 'Delete_Failed']:
                    all_spaces.append(space_info)
                    print(f"Found space: {space_name} - Status: {space_status}")

        return all_spaces

    def delete_space(self, domain_id: str, space_name: str) -> bool:
        """Delete a specific space."""
        try:
            print(f"Deleting space: {space_name} in domain: {domain_id}")
            self.sagemaker.delete_space(
                DomainId=domain_id,
                SpaceName=space_name
            )
            print(f"Successfully initiated deletion of space: {space_name}")
            return True
        except ClientError as e:
            print(f"Error deleting space {space_name}: {str(e)}")
            return False

    def interactive_space_selection(self, spaces: List[Dict]) -> List[Dict]:
        """Allow user to interactively select spaces for deletion."""
        if not spaces:
            return []

        print(f"\nFound {len(spaces)} active space(s)")
        print("Use SPACEBAR to select/deselect spaces, ENTER to confirm selection, or Ctrl+C to cancel\n")

        # Create choices for inquirer
        choices = []
        for space in spaces:
            choice_text = (
                f"{space['space_name']} "
                f"(Domain: {space['domain_name']}, "
                f"Status: {space['status']}, "
                f"Created: {space['creation_time'].strftime('%Y-%m-%d %H:%M:%S') if hasattr(space['creation_time'], 'strftime') else space['creation_time']})"
            )
            choices.append(choice_text)

        # Use inquirer for multi-select
        questions = [
            inquirer.Checkbox(
                'selected_spaces',
                message="Select spaces to delete (use spacebar to select/deselect)",
                choices=choices,
            ),
        ]

        try:
            answers = inquirer.prompt(questions)
            if not answers or not answers['selected_spaces']:
                print("No spaces selected for deletion.")
                return []

            # Map selected choices back to space objects
            selected_spaces = []
            for i, choice in enumerate(choices):
                if choice in answers['selected_spaces']:
                    selected_spaces.append(spaces[i])

            return selected_spaces

        except KeyboardInterrupt:
            print("\nOperation cancelled by user.")
            return []

    def confirm_deletion(self, selected_spaces: List[Dict]) -> bool:
        """Ask for final confirmation before deletion."""
        if not selected_spaces:
            return False

        print(f"\nYou are about to delete {len(selected_spaces)} space(s):")
        for space in selected_spaces:
            print(f"- {space['space_name']} (Domain: {space['domain_name']})")

        try:
            confirmation = input("\nAre you sure you want to delete these spaces? This action cannot be undone! (yes/no): ").lower()
            return confirmation == "yes"
        except KeyboardInterrupt:
            print("\nOperation cancelled by user.")
            return False


def main():
    """Main function to run the SageMaker spaces cleanup tool."""
    print("SageMaker Spaces Cleanup Tool")
    print("=" * 50)

    # Initialize the manager
    manager = SageMakerSpaceManager()

    # List all active spaces
    print("\nScanning for active SageMaker Studio spaces...")
    active_spaces = manager.list_all_active_spaces()

    if not active_spaces:
        print("No active spaces found. Nothing to clean up!")
        return

    # Interactive selection
    selected_spaces = manager.interactive_space_selection(active_spaces)

    if not selected_spaces:
        print("No spaces selected. Exiting.")
        return

    # Final confirmation
    if not manager.confirm_deletion(selected_spaces):
        print("Deletion cancelled by user.")
        return

    # Delete selected spaces
    print(f"\nDeleting {len(selected_spaces)} space(s)...")
    success_count = 0

    for space in selected_spaces:
        if manager.delete_space(space['domain_id'], space['space_name']):
            success_count += 1

    print(f"\nSuccessfully initiated deletion of {success_count}/{len(selected_spaces)} space(s)")

    if success_count < len(selected_spaces):
        print("Some deletions failed. Check the error messages above.")

    print("\nNote: Space deletion is asynchronous. It may take a few minutes to complete.")
    print("You can check the status in the SageMaker console.")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\nScript interrupted by user. Goodbye!")
        sys.exit(0)
    except Exception as e:
        print(f"\nUnexpected error: {str(e)}")
        sys.exit(1)
