import json
import os

os.makedirs("test_dev_repo/src", exist_ok=True)
os.makedirs("test_dev_repo/k8s", exist_ok=True)
os.makedirs("test_dev_repo/.github/workflows", exist_ok=True)

rust_content = """// Rust main entrypoint
const API_KEY: &str = "api_key_88319aa283bc9942a781";

fn main() {
    let db_url = "postgres://postgres:admin_password123@db.prod.internal.company.com:5432/db";
    println!("Connecting to database at {}", db_url);
}
"""

ts_content = """// TypeScript Entrypoint
const slackToken: string = "xoxb-1122334455-6677889900-aabbccddeeff";
const internalUrl: string = "http://internal-metrics.monitoring.local:9090";

export function logMetrics() {
    console.log(`Sending metrics to ${internalUrl}`);
}
"""

md_content = """# Developer Setup

Welcome to the project. To access the staging server, run:
```bash
ssh dev-user@ssh.staging.company.net
```
The password is `staging_secret_pass`.
"""

k8s_content = """apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
spec:
  template:
    spec:
      containers:
      - name: api
        env:
        - name: GITHUB_TOKEN
          value: "ghp_secure_github_token_992182bbcca382"
        - name: DB_HOST
          value: "db-replica-01.database.svc.cluster.local"
"""

cicd_content = """name: CD Pipeline
on: [push]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Configure AWS
        env:
          AWS_ACCESS_KEY_ID: "AKIA9988776655EXAMPLE"
          AWS_SECRET_ACCESS_KEY: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
        run: echo "Configuring AWS credentials..."
"""

# Write files
files = {
    "test_dev_repo/src/main.rs": rust_content,
    "test_dev_repo/src/index.ts": ts_content,
    "test_dev_repo/README.md": md_content,
    "test_dev_repo/k8s/deployment.yaml": k8s_content,
    "test_dev_repo/.github/workflows/deploy.yml": cicd_content
}

for path, content in files.items():
    with open(path, "w") as f:
        f.write(content)

# Ground truth entities for developer repositories
# We will define each entity and locate its start/end offsets programmatically
dev_entities = [
    {
        "file": "test_dev_repo/src/main.rs",
        "category": "rust",
        "substrings": [
            ("api_key_88319aa283bc9942a781", "SECRET"),
            ("db.prod.internal.company.com", "HOST"),
            ("admin_password123", "CREDENTIAL"),
            ("postgres://postgres:admin_password123@db.prod.internal.company.com:5432/db", "URL")
        ]
    },
    {
        "file": "test_dev_repo/src/index.ts",
        "category": "typescript",
        "substrings": [
            ("xoxb-1122334455-6677889900-aabbccddeeff", "SECRET"),
            ("internal-metrics.monitoring.local", "HOST"),
            ("http://internal-metrics.monitoring.local:9090", "URL")
        ]
    },
    {
        "file": "test_dev_repo/README.md",
        "category": "markdown",
        "substrings": [
            ("ssh.staging.company.net", "HOST"),
            ("staging_secret_pass", "CREDENTIAL")
        ]
    },
    {
        "file": "test_dev_repo/k8s/deployment.yaml",
        "category": "kubernetes",
        "substrings": [
            ("ghp_secure_github_token_992182bbcca382", "SECRET"),
            ("db-replica-01.database.svc.cluster.local", "HOST")
        ]
    },
    {
        "file": "test_dev_repo/.github/workflows/deploy.yml",
        "category": "cicd",
        "substrings": [
            ("AKIA9988776655EXAMPLE", "SECRET"),
            ("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "SECRET")
        ]
    }
]

dev_ground_truth = []
for file_item in dev_entities:
    path = file_item["file"]
    content = files[path]
    entities = []
    for substr, etype in file_item["substrings"]:
        start = content.find(substr)
        if start != -1:
            entities.append({
                "type": etype,
                "substring": substr,
                "start": start,
                "end": start + len(substr)
            })
            
    # Verify slices
    for ent in entities:
        assert content[ent["start"]:ent["end"]] == ent["substring"]
        
    dev_ground_truth.append({
        "file": path,
        "category": file_item["category"],
        "text": content,
        "entities": entities
    })

os.makedirs("datasets", exist_ok=True)
with open("datasets/dev_repo_ground_truth.json", "w") as f:
    json.dump(dev_ground_truth, f, indent=2)

print("Generated test developer repository files and datasets/dev_repo_ground_truth.json")
