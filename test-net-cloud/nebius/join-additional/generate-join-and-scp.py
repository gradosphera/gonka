import csv
import os
from pathlib import Path


class Node:
    def __init__(
        self,
        domain: str,
        ssh_port: int,
        p2p_port: int,
        api_port: int,
        user: str,
        ssh_key_path: str,
        deploy_dir: str,
        key_name: str = None,
        hf_home: str = None,
        custom_base_dir: str = None
    ):
        self.domain = domain
        self.ssh_port = ssh_port
        self.p2p_port = p2p_port
        self.api_port = api_port
        self.hf_home = hf_home
        self.user = user
        self.ssh_key_path = ssh_key_path
        self.deploy_dir = deploy_dir
        self.custom_base_dir = custom_base_dir
        self.key_name = key_name or f"join-{self.ssh_port}"

    # based on example from join-1.sh
    def generate_join_script(self, branch: str = "origin/testnet/main", sync_with_snapshots: str = "false"):
        """Generate a join script similar to join-1.sh"""
        script_lines = [
            f'export KEY_NAME="{self.key_name}"',
            f'export PUBLIC_URL="http://{self.domain}:{self.api_port}"',
            f'export P2P_EXTERNAL_ADDRESS="tcp://{self.domain}:{self.p2p_port}"',
            f'export SYNC_WITH_SNAPSHOTS="{sync_with_snapshots}"',
            f'export HF_HOME="{self.hf_home}"',
        ]
        
        if self.custom_base_dir:
            script_lines.append(f'export TESTNET_BASE_DIR="{self.custom_base_dir}"')
        
        script_lines.append(f'python3 launch.py --mode join --branch {branch}')
        
        return '\n'.join(script_lines) + '\n'

    def create_join_script(self, branch: str, sync_with_snapshots: str):
        path = f"{self.deploy_dir}/{self.ssh_port}.sh"

        with open(path, "w") as f:
            f.write(self.generate_join_script(branch, sync_with_snapshots))
        
        # Make the script executable
        os.chmod(path, 0o755)



# based on prepare.sh, all nodes are join
def create_prepare_script(nodes: list[Node], output_dir: str):
    path = f"{output_dir}/prepare.sh"
    
    script_lines = [
        '#!/bin/bash',
        'if [ -n "$SSH_KEY_PATH" ]; then',
        '  SSH_KEY_ARG="-i $SSH_KEY_PATH"',
        'else',
        '  SSH_KEY_ARG=""',
        'fi',
        ''
    ]
    
    for node in nodes:
        target_dir = f"{node.custom_base_dir}/" if node.custom_base_dir else "~/"
        # Copy launch.py
        script_lines.append(f"scp $SSH_KEY_ARG -P {node.ssh_port} launch.py {node.user}@{node.domain}:{target_dir}")
        # Copy join script and rename to join.sh
        script_lines.append(f"scp $SSH_KEY_ARG -P {node.ssh_port} {output_dir}/{node.ssh_port}.sh {node.user}@{node.domain}:{target_dir}join.sh")
    
    with open(path, "w") as f:
        f.write('\n'.join(script_lines) + '\n')
    
    # Make the script executable
    os.chmod(path, 0o755)


if __name__ == "__main__":
    # Configuration from environment and params.csv
    OUTPUT_DIR = "join-additional"  # Local directory for generated scripts
    PARAMS_CSV = f"{OUTPUT_DIR}/params.csv"
    USER = os.environ.get("USER", "ubuntu")
    SSH_KEY_PATH = os.environ.get("SSH_KEY_PATH")  # No default, like prepare.sh
    BRANCH = os.environ.get("BRANCH", "origin/testnet/main")
    SYNC_WITH_SNAPSHOTS = os.environ.get("SYNC_WITH_SNAPSHOTS", "false")
    
    # Create output directory if it doesn't exist
    Path(OUTPUT_DIR).mkdir(parents=True, exist_ok=True)
    
    # 1. Read params.csv
    nodes = []
    with open(PARAMS_CSV, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            node = Node(
                domain=row['domain'],
                ssh_port=int(row['ssh_port']),
                p2p_port=int(row['p2p_port']),
                api_port=int(row['api_port']),
                user=USER,
                ssh_key_path=SSH_KEY_PATH,
                deploy_dir=OUTPUT_DIR,
                key_name=f"join-{row['ssh_port']}",
                hf_home=row['hf_home'].strip(),
                custom_base_dir=row['deploy_dir'].strip()
            )
            nodes.append(node)
            print(f"Created node: {node.domain}:{node.ssh_port} (deploy_dir={node.custom_base_dir}, hf_home={node.hf_home})")
    
    # 2. Generate join scripts for each node
    for node in nodes:
        node.create_join_script(BRANCH, SYNC_WITH_SNAPSHOTS)
        print(f"Generated join script: {OUTPUT_DIR}/{node.ssh_port}.sh")
    
    # 3. Generate prepare.sh (scp script)
    create_prepare_script(nodes, OUTPUT_DIR)
    print(f"Generated prepare script: {OUTPUT_DIR}/prepare.sh")
    
    print(f"\nDone! Generated {len(nodes)} join scripts and 1 prepare script in {OUTPUT_DIR}/")
