#  https://github.com/towardsthecloud/aws-toolbox
#
#  License: MIT
#
# This script displays the AWS organization structure as a Mermaid diagram.
#
# It recursively fetches all organizational units (OUs) and accounts from your AWS
# Organization, then generates a hierarchical Mermaid diagram showing the complete structure.
# The diagram includes the root, all OUs, and active accounts within each OU.
#
# Features:
#   - Generates a visually styled Mermaid flowchart diagram
#   - Shows hierarchical relationships between root, OUs, and accounts
#   - Limits accounts per OU to 6 by default (configurable via MAX_ACCOUNTS_PER_OU)
#   - Automatically copies diagram to clipboard on macOS
#   - Output can be pasted directly into Mermaid Live Editor or GitHub markdown
#
# Input:
#   - No command-line arguments required
#   - Uses default AWS credentials and region from your environment
#
# Output:
#   - Mermaid diagram code (printed to stdout)
#   - Progress messages and instructions (printed to stderr)
#   - Diagram automatically copied to clipboard (macOS only)
#
# Examples:
#   # Generate organization structure diagram
#   python org_display_structure.py
#
#   # Pipe output to a file
#   python org_display_structure.py > org_structure.mmd
#
#   # Use with specific AWS profile
#   AWS_PROFILE=production python org_display_structure.py
#
# View the generated diagram at:
#   - https://mermaid.live/
#   - GitHub (paste in ```mermaid code blocks)

import subprocess
import sys

import boto3

# Configuration
MAX_ACCOUNTS_PER_OU = 6


def sanitize_id(id_string):
    """Convert AWS ID to a valid Mermaid node ID by removing hyphens"""
    return id_string.replace("-", "")


def escape_name(name):
    """Escape special characters in names for Mermaid"""
    return name.replace('"', '\\"')


def get_organizational_units(parent_id, organizations_client):
    """Recursively get all OUs under a parent"""
    ous = []
    paginator = organizations_client.get_paginator("list_organizational_units_for_parent")

    for page in paginator.paginate(ParentId=parent_id):
        for ou in page["OrganizationalUnits"]:
            ou_info = {"Id": ou["Id"], "Name": ou["Name"], "Children": [], "Accounts": []}

            # Get child OUs recursively
            ou_info["Children"] = get_organizational_units(ou["Id"], organizations_client)

            # Get accounts in this OU
            ou_info["Accounts"] = get_accounts_in_ou(ou["Id"], organizations_client)

            ous.append(ou_info)

    return ous


def get_accounts_in_ou(parent_id, organizations_client, max_accounts=MAX_ACCOUNTS_PER_OU):
    """Get accounts in an OU (limited to max_accounts)"""
    accounts = []
    paginator = organizations_client.get_paginator("list_accounts_for_parent")
    total_count = 0

    for page in paginator.paginate(ParentId=parent_id):
        for account in page["Accounts"]:
            if account["Status"] == "ACTIVE":
                total_count += 1
                if len(accounts) < max_accounts:
                    accounts.append(
                        {"Id": account["Id"], "Name": account.get("Alias", account["Name"]), "Status": account["Status"]}
                    )

    # Add indicator if there are more accounts
    if total_count > max_accounts:
        accounts.append(
            {
                "Id": f"more-{parent_id}",
                "Name": f"... +{total_count - max_accounts} more",
                "Status": "PLACEHOLDER"
            }
        )

    return accounts


def generate_mermaid_nodes(ou_data, parent_id, is_root=False):
    """Generate Mermaid node definitions and relationships"""
    nodes = []
    relationships = []

    if is_root:
        # Root node
        root_name = ou_data["Name"]
        root_node_id = sanitize_id(ou_data["Id"])
        nodes.append(f'    {root_node_id}["{escape_name(root_name)}<br/>(Root)"]')

        # Root accounts
        for account in ou_data["Accounts"]:
            if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
                acc_id = sanitize_id(account["Id"])
                nodes.append(f'    {acc_id}["{escape_name(account["Name"])}"]')
                relationships.append(f"    {root_node_id} --> {acc_id}")

    # Process OUs
    for ou in ou_data["Children"]:
        ou_id = sanitize_id(ou["Id"])
        ou_name = escape_name(ou["Name"])
        parent_node_id = sanitize_id(parent_id)

        # Create subgraph for OU
        subgraph_id = f"{ou_id}Sub"
        nodes.append(f'    subgraph {subgraph_id}["{ou_name}"]')
        nodes.append("        direction TB")

        # Add accounts in this OU
        for account in ou["Accounts"]:
            if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
                acc_id = sanitize_id(account["Id"])
                acc_name = escape_name(account["Name"])
                nodes.append(f'        {acc_id}["{acc_name}"]')

        # Recursively process child OUs
        if ou["Children"]:
            child_nodes, child_rels = generate_mermaid_nodes_recursive(ou["Children"], ou["Id"])
            nodes.extend([f"        {line}" if not line.startswith("    ") else line for line in child_nodes])
            relationships.extend(child_rels)

        nodes.append("    end")

        # Add relationship from parent to this OU subgraph
        relationships.append(f"    {parent_node_id} --> {subgraph_id}")

    return nodes, relationships


def generate_mermaid_nodes_recursive(ous, parent_id):
    """Helper function for recursive OU processing"""
    nodes = []
    relationships = []

    for ou in ous:
        ou_id = sanitize_id(ou["Id"])
        ou_name = escape_name(ou["Name"])

        # Create subgraph for child OU
        subgraph_id = f"{ou_id}Sub"
        nodes.append(f'subgraph {subgraph_id}["{ou_name}"]')
        nodes.append("    direction TB")

        # Add accounts in this OU
        for account in ou["Accounts"]:
            if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
                acc_id = sanitize_id(account["Id"])
                acc_name = escape_name(account["Name"])
                nodes.append(f'    {acc_id}["{acc_name}"]')

        # Recursively process child OUs
        if ou["Children"]:
            child_nodes, child_rels = generate_mermaid_nodes_recursive(ou["Children"], ou["Id"])
            nodes.extend([f"    {line}" if not line.startswith("    ") else line for line in child_nodes])
            relationships.extend(child_rels)

        nodes.append("end")

        # No parent relationship here, handled by parent

    return nodes, relationships


def generate_mermaid_diagram(org_structure):
    """Generate complete Mermaid diagram"""
    lines = ["graph TB"]

    root_id = org_structure["Id"]
    root_node_id = sanitize_id(root_id)
    root_name = escape_name(org_structure["Name"])

    # Root node
    lines.append(f'    {root_node_id}["{root_name}<br/>(Root)"]')
    lines.append("")

    # Root accounts
    root_accounts = []
    prev_acc_id = None
    for account in org_structure["Accounts"]:
        if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
            acc_id = sanitize_id(account["Id"])
            acc_name = escape_name(account["Name"])
            root_accounts.append(acc_id)
            lines.append(f'    {acc_id}["{acc_name}"]')
            # Chain accounts vertically
            if prev_acc_id:
                lines.append(f'    {prev_acc_id} ~~~ {acc_id}')
            prev_acc_id = acc_id

    if root_accounts:
        lines.append("")

    # Process OUs
    for ou in org_structure["Children"]:
        ou_id = sanitize_id(ou["Id"])
        ou_name = escape_name(ou["Name"])
        subgraph_id = f"{ou_id}Sub"

        lines.append(f'    subgraph {subgraph_id}["{ou_name}"]')
        lines.append("        direction TB")

        # Add accounts in this OU
        prev_acc_id = None
        for account in ou["Accounts"]:
            if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
                acc_id = sanitize_id(account["Id"])
                acc_name = escape_name(account["Name"])
                lines.append(f'        {acc_id}["{acc_name}"]')
                # Chain accounts vertically
                if prev_acc_id:
                    lines.append(f'        {prev_acc_id} ~~~ {acc_id}')
                prev_acc_id = acc_id

        # Recursively process child OUs
        if ou["Children"]:
            child_nodes, _ = generate_child_ous(ou["Children"], 2)
            lines.extend(child_nodes)

        lines.append("    end")
        lines.append("")

    # Add relationships
    lines.append("")
    for ou in org_structure["Children"]:
        ou_id = sanitize_id(ou["Id"])
        subgraph_id = f"{ou_id}Sub"
        lines.append(f"    {root_node_id} --> {subgraph_id}")

    # Root account relationships
    for acc_id in root_accounts:
        lines.append(f"    {root_node_id} --> {acc_id}")

    lines.append("")

    # Collect all account IDs and OU subgraph IDs for styling
    all_accounts = []
    all_ous = []

    def collect_ids(ou_list):
        for ou in ou_list:
            all_ous.append(f"{sanitize_id(ou['Id'])}Sub")
            for account in ou["Accounts"]:
                if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
                    all_accounts.append(sanitize_id(account["Id"]))
            if ou["Children"]:
                collect_ids(ou["Children"])

    collect_ids(org_structure["Children"])

    # Add root accounts
    for account in org_structure["Accounts"]:
        if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
            all_accounts.append(sanitize_id(account["Id"]))

    # Add styles
    lines.append("    classDef rootStyle fill:#ff9999,stroke:#cc0000,stroke-width:3px,color:#000")
    lines.append("    classDef ouStyle fill:#fff,stroke:#e91e63,stroke-width:2px,stroke-dasharray: 5 5,color:#000")
    lines.append("    classDef accountStyle fill:#fff,stroke:#e91e63,stroke-width:2px,color:#000")
    lines.append("    linkStyle default stroke:#e91e63,stroke-width:2px")
    lines.append("")
    lines.append(f"    class {root_node_id} rootStyle")

    if all_ous:
        lines.append(f"    class {','.join(all_ous)} ouStyle")
    if all_accounts:
        lines.append(f"    class {','.join(all_accounts)} accountStyle")

    return "\n".join(lines)


def generate_child_ous(ous, indent_level):
    """Recursively generate child OU subgraphs"""
    lines = []
    indent = "    " * indent_level

    for ou in ous:
        ou_id = sanitize_id(ou["Id"])
        ou_name = escape_name(ou["Name"])
        subgraph_id = f"{ou_id}Sub"

        lines.append(f'{indent}subgraph {subgraph_id}["{ou_name}"]')
        lines.append(f"{indent}    direction TB")

        # Add accounts
        prev_acc_id = None
        for account in ou["Accounts"]:
            if account["Status"] in ("ACTIVE", "PLACEHOLDER"):
                acc_id = sanitize_id(account["Id"])
                acc_name = escape_name(account["Name"])
                lines.append(f'{indent}    {acc_id}["{acc_name}"]')
                # Chain accounts vertically
                if prev_acc_id:
                    lines.append(f'{indent}    {prev_acc_id} ~~~ {acc_id}')
                prev_acc_id = acc_id

        # Recursively process children
        if ou["Children"]:
            child_lines, _ = generate_child_ous(ou["Children"], indent_level + 1)
            lines.extend(child_lines)

        lines.append(f"{indent}end")

    return lines, []


def main():
    """Main function"""
    try:
        # Create AWS Organizations client
        organizations = boto3.client("organizations")

        # Get root
        response = organizations.list_roots()
        root = response["Roots"][0]
        root_id = root["Id"]
        root_name = root.get("Name", "AWS Organization")

        print("Fetching organization structure...", file=sys.stderr)

        # Build organization structure
        org_structure = {
            "Id": root_id,
            "Name": root_name,
            "Accounts": get_accounts_in_ou(root_id, organizations),
            "Children": get_organizational_units(root_id, organizations),
        }

        print("Generating Mermaid diagram...\n", file=sys.stderr)

        # Generate and print Mermaid diagram
        mermaid_diagram = generate_mermaid_diagram(org_structure)
        print(mermaid_diagram)

        # Copy to clipboard
        try:
            process = subprocess.Popen(['pbcopy'], stdin=subprocess.PIPE)
            process.communicate(mermaid_diagram.encode('utf-8'))
            print("\n\n✓ Diagram copied to clipboard!", file=sys.stderr)
        except FileNotFoundError:
            print("\n\nNote: pbcopy not available (macOS only)", file=sys.stderr)

        print("\nPaste the diagram into a Mermaid viewer:", file=sys.stderr)
        print("- https://mermaid.live/", file=sys.stderr)
        print("- GitHub Markdown (in ```mermaid code blocks)", file=sys.stderr)
        print(f"\nShowing max {MAX_ACCOUNTS_PER_OU} accounts per OU", file=sys.stderr)

    except Exception as e:
        print(f"Error: {str(e)}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
