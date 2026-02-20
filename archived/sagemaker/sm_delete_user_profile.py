#!/usr/bin/env python3
"""
SageMaker User Profile Deletion Tool

This script lists all SageMaker Studio user profiles and allows you to
interactively select which one to delete. It shows all corresponding apps
and spaces that must be deleted first before the user profile can be removed.

Requirements:
- boto3
- inquirer
- AWS credentials configured

Usage:
    pip install -r requirements.txt
    python sm_delete_user_profile.py

Author: Danny Steenman
License: MIT
"""

import sys
import time
from typing import Dict, List, Optional

import boto3
import inquirer
from botocore.exceptions import ClientError, NoCredentialsError


class SageMakerUserProfileManager:
    def __init__(self, region_name: Optional[str] = None):
        """Initialize the SageMaker client."""
        try:
            self.session = boto3.Session()
            self.sagemaker = self.session.client("sagemaker", region_name=region_name)
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
            paginator = self.sagemaker.get_paginator("list_domains")
            domains = []
            for page in paginator.paginate():
                domains.extend(page.get("Domains", []))
            return domains
        except ClientError as e:
            print(f"Error listing domains: {str(e)}")
            return []

    def list_user_profiles_in_domain(self, domain_id: str) -> List[Dict]:
        """List all user profiles in a specific domain."""
        try:
            paginator = self.sagemaker.get_paginator("list_user_profiles")
            user_profiles = []
            for page in paginator.paginate(DomainIdEquals=domain_id):
                user_profiles.extend(page.get("UserProfiles", []))
            return user_profiles
        except ClientError as e:
            print(f"Error listing user profiles in domain {domain_id}: {str(e)}")
            return []

    def list_all_user_profiles(self) -> List[Dict]:
        """List all user profiles across all domains."""
        all_profiles = []
        domains = self.list_domains()

        if not domains:
            print("No SageMaker Studio domains found in this region.")
            return []

        print(f"Found {len(domains)} domain(s). Scanning for user profiles...")

        for domain in domains:
            domain_id = domain["DomainId"]
            domain_name = domain.get("DomainName", "Unknown")
            print(f"Checking domain: {domain_name} ({domain_id})")

            user_profiles = self.list_user_profiles_in_domain(domain_id)

            for profile in user_profiles:
                profile_name = profile["UserProfileName"]
                profile_status = profile.get("Status", "Unknown")

                profile_info = {
                    "domain_id": domain_id,
                    "domain_name": domain_name,
                    "user_profile_name": profile_name,
                    "status": profile_status,
                    "creation_time": profile.get("CreationTime", "Unknown"),
                }

                # Only include profiles that are not being deleted
                if profile_status not in ["Deleting", "Delete_Failed"]:
                    all_profiles.append(profile_info)
                    print(f"Found user profile: {profile_name} - Status: {profile_status}")

        return all_profiles

    def list_apps_for_user_profile(self, domain_id: str, user_profile_name: str) -> List[Dict]:
        """List all apps for a specific user profile."""
        try:
            paginator = self.sagemaker.get_paginator("list_apps")
            apps = []
            for page in paginator.paginate(DomainIdEquals=domain_id, UserProfileNameEquals=user_profile_name):
                apps.extend(page.get("Apps", []))
            return apps
        except ClientError as e:
            print(f"Error listing apps for user profile {user_profile_name}: {str(e)}")
            return []

    def list_spaces_for_user_profile(self, domain_id: str, user_profile_name: str) -> List[Dict]:
        """List all spaces owned by a specific user profile."""
        try:
            paginator = self.sagemaker.get_paginator("list_spaces")
            spaces = []
            for page in paginator.paginate(DomainIdEquals=domain_id):
                for space in page.get("Spaces", []):
                    # Check if this space belongs to the user profile
                    space_details = self.get_space_details(domain_id, space["SpaceName"])
                    if space_details.get("OwnershipSettings", {}).get("OwnerUserProfileName") == user_profile_name:
                        spaces.append(space)
            return spaces
        except ClientError as e:
            print(f"Error listing spaces for user profile {user_profile_name}: {str(e)}")
            return []

    def get_space_details(self, domain_id: str, space_name: str) -> Dict:
        """Get detailed information about a specific space."""
        try:
            response = self.sagemaker.describe_space(DomainId=domain_id, SpaceName=space_name)
            return response
        except ClientError as e:
            print(f"Error getting details for space {space_name}: {str(e)}")
            return {}

    def delete_app(self, domain_id: str, user_profile_name: str, app_type: str, app_name: str) -> bool:
        """Delete a specific app."""
        try:
            print(f"Deleting app: {app_name} (type: {app_type}) for user: {user_profile_name}")
            self.sagemaker.delete_app(
                DomainId=domain_id, UserProfileName=user_profile_name, AppType=app_type, AppName=app_name
            )
            print(f"Successfully initiated deletion of app: {app_name}")
            return True
        except ClientError as e:
            print(f"Error deleting app {app_name}: {str(e)}")
            return False

    def delete_space(self, domain_id: str, space_name: str) -> bool:
        """Delete a specific space."""
        try:
            print(f"Deleting space: {space_name}")
            self.sagemaker.delete_space(DomainId=domain_id, SpaceName=space_name)
            print(f"Successfully initiated deletion of space: {space_name}")
            return True
        except ClientError as e:
            print(f"Error deleting space {space_name}: {str(e)}")
            return False

    def delete_user_profile(self, domain_id: str, user_profile_name: str) -> bool:
        """Delete a specific user profile."""
        try:
            print(f"Deleting user profile: {user_profile_name}")
            self.sagemaker.delete_user_profile(DomainId=domain_id, UserProfileName=user_profile_name)
            print(f"Successfully initiated deletion of user profile: {user_profile_name}")
            return True
        except ClientError as e:
            print(f"Error deleting user profile {user_profile_name}: {str(e)}")
            return False

    def wait_for_apps_deletion(self, domain_id: str, user_profile_name: str, apps: List[Dict]) -> bool:
        """Wait for all apps to be deleted before proceeding."""
        if not apps:
            return True

        print("Waiting for apps to be deleted...")
        max_wait_time = 300  # 5 minutes
        wait_interval = 10  # 10 seconds
        elapsed_time = 0

        while elapsed_time < max_wait_time:
            remaining_apps = self.list_apps_for_user_profile(domain_id, user_profile_name)
            active_apps = [app for app in remaining_apps if app.get("Status") not in ["Deleted", "Failed"]]

            if not active_apps:
                print("All apps have been deleted.")
                return True

            print(f"Waiting for {len(active_apps)} app(s) to be deleted... ({elapsed_time}s elapsed)")
            time.sleep(wait_interval)
            elapsed_time += wait_interval

        print("Timeout waiting for apps to be deleted. Please check manually.")
        return False

    def wait_for_spaces_deletion(self, domain_id: str, user_profile_name: str, spaces: List[Dict]) -> bool:
        """Wait for all spaces to be deleted before proceeding."""
        if not spaces:
            return True

        print("Waiting for spaces to be deleted...")
        max_wait_time = 600  # 10 minutes (spaces can take longer than apps)
        wait_interval = 15  # 15 seconds
        elapsed_time = 0

        while elapsed_time < max_wait_time:
            remaining_spaces = self.list_spaces_for_user_profile(domain_id, user_profile_name)
            active_spaces = [space for space in remaining_spaces if space.get("Status") not in ["Deleted", "Failed"]]

            if not active_spaces:
                print("All spaces have been deleted.")
                return True

            print(f"Waiting for {len(active_spaces)} space(s) to be deleted... ({elapsed_time}s elapsed)")
            for space in active_spaces:
                print(f"  - {space['SpaceName']}: {space.get('Status', 'Unknown')}")

            time.sleep(wait_interval)
            elapsed_time += wait_interval

        print("Timeout waiting for spaces to be deleted. Please check manually.")
        return False

    def interactive_user_profile_selection(self, profiles: List[Dict]) -> Optional[Dict]:
        """Allow user to interactively select a user profile for deletion."""
        if not profiles:
            return None

        print(f"\nFound {len(profiles)} user profile(s)")
        print("Use arrow keys to navigate, ENTER to select, or Ctrl+C to cancel\n")

        # Create choices for inquirer
        choices = []
        for profile in profiles:
            choice_text = (
                f"{profile['user_profile_name']} "
                f"(Domain: {profile['domain_name']}, "
                f"Status: {profile['status']}, "
                f"Created: {profile['creation_time'].strftime('%Y-%m-%d %H:%M:%S') if hasattr(profile['creation_time'], 'strftime') else profile['creation_time']})"
            )
            choices.append(choice_text)

        # Use inquirer for single select
        questions = [
            inquirer.List(
                "selected_profile",
                message="Select user profile to delete",
                choices=choices,
            ),
        ]

        try:
            answers = inquirer.prompt(questions)
            if not answers or not answers["selected_profile"]:
                print("No user profile selected.")
                return None

            # Map selected choice back to profile object
            for i, choice in enumerate(choices):
                if choice == answers["selected_profile"]:
                    return profiles[i]

            return None

        except KeyboardInterrupt:
            print("\nOperation cancelled by user.")
            return None

    def show_dependencies_and_confirm(self, profile: Dict, apps: List[Dict], spaces: List[Dict]) -> bool:
        """Show all dependencies and ask for confirmation to delete everything."""
        print(f"\nUser Profile: {profile['user_profile_name']} (Domain: {profile['domain_name']})")
        print("=" * 60)

        print(f"\nFound {len(apps)} app(s):")
        if apps:
            for app in apps:
                print(f"- {app['AppName']} (Type: {app['AppType']}, Status: {app.get('Status', 'Unknown')})")
        else:
            print("- No apps found")

        print(f"\nFound {len(spaces)} space(s):")
        if spaces:
            for space in spaces:
                print(f"- {space['SpaceName']} (Status: {space.get('Status', 'Unknown')})")
        else:
            print("- No spaces found")

        if not apps and not spaces:
            print("\nNo dependencies found. User profile can be deleted directly.")
        else:
            print(
                f"\nTo delete this user profile, all {len(apps)} app(s) and {len(spaces)} space(s) must be deleted first."
            )

        try:
            confirmation = input(
                f"\nDo you want to delete all dependencies and the user profile '{profile['user_profile_name']}'? (yes/no): "
            ).lower()
            return confirmation == "yes"
        except KeyboardInterrupt:
            print("\nOperation cancelled by user.")
            return False


def main():
    """Main function to run the SageMaker user profile deletion tool."""
    print("SageMaker User Profile Deletion Tool")
    print("=" * 50)

    # Initialize the manager
    manager = SageMakerUserProfileManager()

    # List all user profiles
    print("\nScanning for SageMaker Studio user profiles...")
    user_profiles = manager.list_all_user_profiles()

    if not user_profiles:
        print("No user profiles found. Nothing to delete!")
        return

    # Interactive selection
    selected_profile = manager.interactive_user_profile_selection(user_profiles)

    if not selected_profile:
        print("No user profile selected. Exiting.")
        return

    # Get dependencies (apps and spaces)
    print(f"\nAnalyzing dependencies for user profile: {selected_profile['user_profile_name']}")

    apps = manager.list_apps_for_user_profile(selected_profile["domain_id"], selected_profile["user_profile_name"])

    spaces = manager.list_spaces_for_user_profile(selected_profile["domain_id"], selected_profile["user_profile_name"])

    # Show dependencies and get confirmation
    if not manager.show_dependencies_and_confirm(selected_profile, apps, spaces):
        print("Deletion cancelled by user.")
        return

    # Delete dependencies first
    success_count = 0
    total_items = len(apps) + len(spaces)

    # Delete apps first
    if apps:
        print(f"\nDeleting {len(apps)} app(s)...")
        for app in apps:
            if manager.delete_app(
                selected_profile["domain_id"], selected_profile["user_profile_name"], app["AppType"], app["AppName"]
            ):
                success_count += 1

        # Wait for apps to be deleted
        if not manager.wait_for_apps_deletion(
            selected_profile["domain_id"], selected_profile["user_profile_name"], apps
        ):
            print("Failed to wait for apps deletion. Please check manually before proceeding.")
            return

    # Delete spaces
    if spaces:
        print(f"\nDeleting {len(spaces)} space(s)...")
        for space in spaces:
            if manager.delete_space(selected_profile["domain_id"], space["SpaceName"]):
                success_count += 1

        # Wait for spaces to be deleted
        if not manager.wait_for_spaces_deletion(
            selected_profile["domain_id"], selected_profile["user_profile_name"], spaces
        ):
            print("Failed to wait for spaces deletion. Please check manually before proceeding.")
            return

    # Delete user profile
    if total_items == 0 or success_count == total_items:
        print(f"\nDeleting user profile: {selected_profile['user_profile_name']}")
        if manager.delete_user_profile(selected_profile["domain_id"], selected_profile["user_profile_name"]):
            print(f"\nSuccessfully initiated deletion of user profile: {selected_profile['user_profile_name']}")
        else:
            print("Failed to delete user profile.")
    else:
        print(f"\nFailed to delete some dependencies ({success_count}/{total_items}). User profile not deleted.")
        print("Please resolve the issues and try again.")

    print("\nNote: Deletion is asynchronous. It may take several minutes to complete.")
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
