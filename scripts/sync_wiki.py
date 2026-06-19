#!/usr/bin/env python3
import os
import re
import shutil

# Paths
SRC_DIR = "apps/web/public/content"
DIST_SECRETS = "apps/web/dist/content/SECRETS.md"
WIKI_DIR = "ocultar-wiki"

# Navigation Structure matching Docs.tsx
SECTIONS = [
    {
        "label": "Guides",
        "items": [
            { "label": "Setup Guide", "name": "SETUP_GUIDE" },
            { "label": "French Finance Quickstart", "name": "FRENCH_FINANCE_QUICKSTART" },
            { "label": "Developer Guide", "name": "DEVELOPER_GUIDE" },
            { "label": "Enterprise Setup", "name": "ENTERPRISE_SETUP_GUIDE" },
            { "label": "MCP Extensions", "name": "MCP_EXTENSIONS" },
            { "label": "Connectors", "name": "CONNECTORS_GUIDE" },
            { "label": "Refinery Proxy", "name": "refinery_proxy_setup" },
            { "label": "Sombra Guide", "name": "SOMBRA_GUIDE" },
            { "label": "Testing Guide", "name": "TESTING_GUIDE" },
            { "label": "Release Guide", "name": "RELEASE_GUIDE" },
        ],
    },
    {
        "label": "API",
        "items": [
            { "label": "API Reference", "name": "API_REFERENCE" },
        ],
    },
    {
        "label": "Reference",
        "items": [
            { "label": "Architecture", "name": "ARCHITECTURE" },
            { "label": "PII Detection", "name": "PII_DETECTION" },
            { "label": "FAQ", "name": "FAQ" },
            { "label": "Product Context", "name": "PRODUCT_CONTEXT" },
            { "label": "EU Sovereign Pack", "name": "EU_SOVEREIGN_PACK_V1" },
        ],
    },
    {
        "label": "Compliance & Security",
        "items": [
            { "label": "GDPR / Privacy by Design", "name": "GDPR_PRIVACY_BY_DESIGN" },
            { "label": "GDPR — French Finance (DPO)", "name": "GDPR_FRENCH_FINANCE" },
            { "label": "Security Model", "name": "SECURITY_MODEL" },
        ],
    },
    {
        "label": "Other",
        "items": [
            { "label": "Secret Management", "name": "SECRETS" },
            { "label": "Blog: Zero-Egress Supply Chain", "name": "zero-egress-supply-chain" },
        ],
    },
    {
        "label": "More Resources",
        "items": [
            { "label": "Entity Registry Guide", "name": "ENTITY_REGISTRY_GUIDE" },
            { "label": "Vault Guide", "name": "VAULT_GUIDE" },
            { "label": "Onboarding Guide", "name": "ONBOARDING_GUIDE" },
            { "label": "Privacy Filter Evaluation", "name": "privacy_filter_eval" },
            { "label": "Skills Summary", "name": "skills_summary" },
        ],
    }
]

def clean_wiki_dir(wiki_dir):
    """Clean all files and directories in wiki_dir except .git."""
    print(f"Cleaning wiki directory: {wiki_dir}...")
    if not os.path.exists(wiki_dir):
        print(f"Error: {wiki_dir} does not exist. Did the git clone fail?")
        return False
    
    for item in os.listdir(wiki_dir):
        if item == ".git":
            continue
        path = os.path.join(wiki_dir, item)
        if os.path.isdir(path):
            shutil.rmtree(path)
        else:
            os.remove(path)
    return True

def find_md_files(src_dir):
    """Find all markdown files recursively in src_dir."""
    md_files = []
    for root, _, files in os.walk(src_dir):
        for f in files:
            if f.endswith(".md"):
                md_files.append(os.path.join(root, f))
    return md_files

def transform_links(content):
    """Rewrite relative markdown links to match the flat wiki structure."""
    # Pattern to match: [text](link)
    pattern = re.compile(r'\[([^\]]+)\]\(([^)]+)\)')
    
    def replace_link(match):
        text = match.group(1)
        dest = match.group(2)
        
        # Keep external links, mailto links, and anchor-only links as they are
        if dest.startswith(("http://", "https://", "mailto:", "#", "ftp://")):
            return f"[{text}]({dest})"
        
        # Split path and anchor
        parts = dest.split('#')
        path_part = parts[0]
        anchor_part = "#" + parts[1] if len(parts) > 1 else ""
        
        # Get filename and strip extension
        filename = os.path.basename(path_part)
        name_only, ext = os.path.splitext(filename)
        
        if ext.lower() == ".md" or ext == "":
            # Rewritten link to flattened name
            return f"[{text}]({name_only}{anchor_part})"
        else:
            # Keep other resources as-is
            return f"[{text}]({dest})"
            
    return pattern.sub(replace_link, content)

def generate_sidebar(sections, target_path):
    """Generate the _Sidebar.md file."""
    print("Generating _Sidebar.md...")
    lines = ["### [Home](Home)\n"]
    
    for sec in sections:
        lines.append(f"### {sec['label']}")
        for item in sec["items"]:
            lines.append(f"* [{item['label']}]({item['name']})")
        lines.append("") # Empty line after section
        
    with open(target_path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))

def main():
    if not clean_wiki_dir(WIKI_DIR):
        return
        
    # Get all markdown files
    md_files = find_md_files(SRC_DIR)
    
    # Check if SECRETS.md is in source; if not, use the build version if available
    secrets_in_source = any(os.path.basename(f) == "SECRETS.md" for f in md_files)
    if not secrets_in_source:
        if os.path.exists(DIST_SECRETS):
            print(f"Including SECRETS.md from {DIST_SECRETS}")
            md_files.append(DIST_SECRETS)
        else:
            print("Warning: SECRETS.md not found in source or dist. Skipping.")

    print(f"Found {len(md_files)} markdown files to sync.")
    
    # Copy files and rewrite links
    setup_guide_content = None
    for src_path in md_files:
        filename = os.path.basename(src_path)
        dest_path = os.path.join(WIKI_DIR, filename)
        
        print(f"Syncing: {src_path} -> {dest_path}")
        with open(src_path, "r", encoding="utf-8") as f:
            content = f.read()
            
        transformed = transform_links(content)
        
        with open(dest_path, "w", encoding="utf-8") as f:
            f.write(transformed)
            
        # Store SETUP_GUIDE content to make it Home.md later
        if filename == "SETUP_GUIDE.md":
            setup_guide_content = transformed

    # Generate Home.md (copy of SETUP_GUIDE.md or default fallback)
    home_path = os.path.join(WIKI_DIR, "Home.md")
    if setup_guide_content:
        print("Generating Home.md from SETUP_GUIDE.md...")
        with open(home_path, "w", encoding="utf-8") as f:
            f.write(setup_guide_content)
    else:
        print("Warning: SETUP_GUIDE.md not found, creating a basic Home.md...")
        with open(home_path, "w", encoding="utf-8") as f:
            f.write("# Ocultar Wiki\nWelcome to the Ocultar documentation wiki. Please use the sidebar to navigate.")

    # Generate _Sidebar.md
    sidebar_path = os.path.join(WIKI_DIR, "_Sidebar.md")
    generate_sidebar(SECTIONS, sidebar_path)
    
    print("\nSync completed successfully!")

if __name__ == "__main__":
    main()
