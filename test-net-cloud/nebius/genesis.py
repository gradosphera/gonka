import os
import shutil
import hashlib
import urllib.request
import zipfile
import subprocess
import json
import re
import time
from pathlib import Path
from types import SimpleNamespace


BASE_DIR = Path(os.environ["HOME"]).absolute()
GENESIS_VAL_NAME = "testnet-genesis"
GONKA_REPO_DIR = BASE_DIR / "gonka"

INFERENCED_BINARY = SimpleNamespace(
    zip_file=BASE_DIR / "inferenced-linux-amd64.zip",
    url="https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.0/inferenced-linux-amd64.zip",
    checksum="24d4481bee27573b5a852265cf0672e1603e405ae1f1f9fba15a7a986feca569",
    path=BASE_DIR / "inferenced",
)

INFERENCED_STATE_DIR = BASE_DIR / ".inference"

CONFIG_ENV = {
    "KEY_NAME": "genesis", # TODO: allow to customize
    "KEYRING_PASSWORD": "12345678", # TODO: allow to customize
    "API_PORT": "8000",
    "PUBLIC_URL": "http://89.169.111.79:8000", # TODO: allow to customize
    "P2P_EXTERNAL_ADDRESS": "tcp://89.169.111.79:5000", # TODO: allow to customize
    "ACCOUNT_PUBKEY": "", # will be populated later
    "NODE_CONFIG": "./node-config.json",
    "HF_HOME": (Path(os.environ["HOME"]).absolute() / "hf-cache").__str__(),
    "SEED_API_URL": "http://89.169.111.79:8000",
    "SEED_NODE_RPC_URL": "http://89.169.111.79:26657",
    "DAPI_API__POC_CALLBACK_URL": "http://api:9100",
    "DAPI_CHAIN_NODE__URL": "http://node:26657",
    "DAPI_CHAIN_NODE__P2P_URL": "http://node:26656",
    "SEED_NODE_P2P_URL": "tcp://89.169.111.79:5000",
    "RPC_SERVER_URL_1": "http://89.169.111.79:26657",
    "RPC_SERVER_URL_2": "http://89.169.111.79:26657",
    "PORT": "8080",
    "INFERENCE_PORT": "5050",
    "KEYRING_BACKEND": "file",
}

def clean_state():
    if GONKA_REPO_DIR.exists():
        print(f"Removing {GONKA_REPO_DIR}")
        os.system(f"sudo rm -rf {GONKA_REPO_DIR}")
    
    if INFERENCED_BINARY.zip_file.exists():
        print(f"Removing {BASE_DIR / 'inferenced-linux-amd64.zip'}")
        os.system(f"sudo rm -f {BASE_DIR / 'inferenced-linux-amd64.zip'}")
    
    if INFERENCED_BINARY.path.exists():
        print(f"Removing {BASE_DIR / 'inferenced'}")
        os.system(f"sudo rm -f {BASE_DIR / 'inferenced'}")

    if INFERENCED_STATE_DIR.exists():
        print(f"Removing {INFERENCED_STATE_DIR}")
        os.system(f"sudo rm -rf {INFERENCED_STATE_DIR}")

def clone_repo():
    if not GONKA_REPO_DIR.exists():
        print(f"Cloning {GONKA_REPO_DIR}")
        os.system(f"git clone https://github.com/gonka-ai/gonka.git {GONKA_REPO_DIR}")
    else:
        print(f"{GONKA_REPO_DIR} already exists")


def create_state_dirs():
    template_dir = GONKA_REPO_DIR / "genesis/validators/template"
    my_dir = GONKA_REPO_DIR / f"genesis/validators/{GENESIS_VAL_NAME}"
    if not my_dir.exists():
        print(f"Creating {my_dir}")
        os.system(f"cp -r {template_dir} {my_dir}")
    else:
        print(f"{my_dir} already exists, contents: {list(my_dir.iterdir())}")


def install_inferenced():
    url = INFERENCED_BINARY.url
    inferenced_zip = INFERENCED_BINARY.zip_file
    checksum = INFERENCED_BINARY.checksum
    inferenced_path = INFERENCED_BINARY.path

    # Download if not exists
    if not inferenced_zip.exists():
        print(f"Downloading inferenced binary zip: {INFERENCED_BINARY.url}")
        urllib.request.urlretrieve(url, inferenced_zip)
    else:
        print(f"{inferenced_zip} already exists")
    
    # Verify checksum
    print(f"Verifying inferenced binary zip checksum...")
    with open(inferenced_zip, 'rb') as f:
        file_hash = hashlib.sha256(f.read()).hexdigest()
    
    if file_hash != checksum:
        raise ValueError(f"Checksum mismatch! Expected: {checksum}, Got: {file_hash}")
    else:
        print("Checksum verified successfully")
    
    # Extract if directory doesn't exist
    if not inferenced_path.exists():
        print(f"Extracting {inferenced_zip} to {BASE_DIR}")
        with zipfile.ZipFile(inferenced_zip, 'r') as zip_ref:
            zip_ref.extractall(BASE_DIR)
        
        # chmod +x $BASE_DIR/inferenced
        os.chmod(inferenced_path, 0o755)
    else:
        print(f"{inferenced_path} already exists")


def create_account_key():
    """Create account key using inferenced CLI"""
    inferenced_binary = INFERENCED_BINARY.path
    
    if not inferenced_binary.exists():
        raise FileNotFoundError(f"Inferenced binary not found at {inferenced_binary}")
    
    # Check if key already exists
    try:
        result = subprocess.run(
            [str(inferenced_binary), "keys", "list", "--keyring-backend", "file"],
            capture_output=True,
            text=True,
            check=True
        )
        if "gonka-account-key" in result.stdout:
            print("Account key 'gonka-account-key' already exists")
            return
    except subprocess.CalledProcessError:
        # Keyring might not exist yet, which is fine
        pass
    
    print("Creating account key 'gonka-account-key' with auto-generated passphrase...")
    
    # Execute the key creation command with automated password input
    # The password is "12345678" and needs to be entered twice
    password = f"{CONFIG_ENV['KEYRING_PASSWORD']}\n"  # \n for newline
    password_input = password + password  # Enter password twice
    
    process = subprocess.Popen([
        str(inferenced_binary), 
        "keys", 
        "add", 
        "gonka-account-key", 
        "--keyring-backend", 
        "file"
    ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    
    stdout, stderr = process.communicate(input=password_input)
    
    if process.returncode != 0:
        print(f"Error creating key: {stderr}")
        raise subprocess.CalledProcessError(process.returncode, "inferenced keys add")
    
    print("Account key created successfully!")
    print("Key details:")
    print(stdout)
    
    # Extract the public key from the output
    
    # Look for the pubkey line in the output
    pubkey_match = re.search(r"pubkey: '(.+?)'", stdout)
    if pubkey_match:
        pubkey_json = pubkey_match.group(1)
        try:
            pubkey_data = json.loads(pubkey_json)
            public_key = pubkey_data.get("key", "")
            if public_key:
                CONFIG_ENV["ACCOUNT_PUBKEY"] = public_key
                print(f"Extracted public key: {public_key}")
            else:
                print("Warning: Could not extract key from pubkey JSON")
        except json.JSONDecodeError:
            print("Warning: Could not parse pubkey JSON")
    else:
        print("Warning: Could not find pubkey in output")


def create_config_env_file():
    """Create config.env file in deploy/join directory"""
    config_file_path = GONKA_REPO_DIR / "deploy/join/config.env"
    
    # Ensure the directory exists
    config_file_path.parent.mkdir(parents=True, exist_ok=True)
    
    # Create the config.env content
    config_content = []
    for key, value in CONFIG_ENV.items():
        config_content.append(f'export {key}="{value}"')
    
    # Write to file
    with open(config_file_path, 'w') as f:
        f.write('\n'.join(config_content))
    
    print(f"Created config.env at {config_file_path}")
    print("== config.env ==")
    print('\n'.join(config_content))
    print("=============")


def pull_images():
    """Pull Docker images using docker compose"""
    working_dir = GONKA_REPO_DIR / "deploy/join"
    config_file = working_dir / "config.env"
    
    if not working_dir.exists():
        raise FileNotFoundError(f"Working directory not found: {working_dir}")
    
    if not config_file.exists():
        raise FileNotFoundError(f"Config file not found: {config_file}")
    
    print(f"Pulling Docker images from {working_dir}")
    
    # Create the command to source config.env and run docker compose
    # We use bash -c to run both commands in sequence
    cmd = f"bash -c 'source {config_file} && docker compose -f docker-compose.yml -f docker-compose.mlnode.yml pull'"
    
    # Run the command in the specified working directory
    result = subprocess.run(
        cmd,
        shell=True,
        cwd=working_dir,
        capture_output=True,
        text=True
    )
    
    if result.returncode != 0:
        print(f"Error pulling images: {result.stderr}")
        raise subprocess.CalledProcessError(result.returncode, cmd)
    
    print("Docker images pulled successfully!")
    if result.stdout:
        print(result.stdout)


def create_docker_compose_override():
    """Create a docker-compose override file for genesis initialization"""
    working_dir = GONKA_REPO_DIR / "deploy/join"
    override_file = working_dir / "docker-compose.genesis-override.yml"
    
    # Create the override content
    override_content = """services:
  node:
    environment:
      - INIT_ONLY=true
      - IS_GENESIS=true
"""
    
    with open(override_file, 'w') as f:
        f.write(override_content)
    
    print(f"Created docker-compose override at {override_file}")
    return override_file


def run_genesis_initialization():
    """Run the node container with genesis initialization settings"""
    working_dir = GONKA_REPO_DIR / "deploy/join"
    config_file = working_dir / "config.env"
    override_file = create_docker_compose_override()
    
    if not working_dir.exists():
        raise FileNotFoundError(f"Working directory not found: {working_dir}")
    
    if not config_file.exists():
        raise FileNotFoundError(f"Config file not found: {config_file}")
    
    print("Running genesis initialization...")
    print("This will initialize the node with INIT_ONLY=true and IS_GENESIS=true")
    
    # Create the command to source config.env and run docker compose with override
    cmd = f"bash -c 'source {config_file} && docker compose -f docker-compose.yml -f docker-compose.mlnode.yml -f {override_file} run --rm node'"
    
    # Run the command in the specified working directory
    result = subprocess.run(
        cmd,
        shell=True,
        cwd=working_dir,
        capture_output=True,
        text=True
    )
    
    print("Genesis initialization completed!")
    print("Output:")
    print("=" * 50)
    if result.stdout:
        print(result.stdout)
    if result.stderr:
        print("Errors/Warnings:")
        print(result.stderr)
    print("=" * 50)
    
    # Extract nodeId from output
    full_output = result.stdout + result.stderr if result.stderr else result.stdout
    node_id_match = re.search(r'nodeId:\s*([a-f0-9]+)', full_output)
    if node_id_match:
        node_id = node_id_match.group(1)
        print(f"Extracted nodeId: {node_id}")
        # Store in CONFIG_ENV for potential future use
        CONFIG_ENV["NODE_ID"] = node_id
    else:
        print("Warning: Could not extract nodeId from output")
    
    if result.returncode != 0:
        print(f"Genesis initialization failed with return code: {result.returncode}")
        raise subprocess.CalledProcessError(result.returncode, cmd)
    
    print("Genesis initialization completed successfully!")


def extract_consensus_key():
    """Extract consensus key from tmkms container"""
    working_dir = GONKA_REPO_DIR / "deploy/join"
    config_file = working_dir / "config.env"
    
    if not working_dir.exists():
        raise FileNotFoundError(f"Working directory not found: {working_dir}")
    
    if not config_file.exists():
        raise FileNotFoundError(f"Config file not found: {config_file}")
    
    print("Extracting consensus key from tmkms...")
    
    # First, start tmkms container in detached mode
    print("Starting tmkms container...")
    start_cmd = f"bash -c 'source {config_file} && docker compose -f docker-compose.yml -f docker-compose.mlnode.yml up -d tmkms'"
    
    start_result = subprocess.run(
        start_cmd,
        shell=True,
        cwd=working_dir,
        capture_output=True,
        text=True
    )
    
    if start_result.returncode != 0:
        print(f"Error starting tmkms container: {start_result.stderr}")
        raise subprocess.CalledProcessError(start_result.returncode, start_cmd)
    
    print("Tmkms container started successfully")
    
    # Wait a moment for container to be ready
    time.sleep(2)
    
    # Now run the tmkms-pubkey command
    print("Running tmkms-pubkey command...")
    pubkey_cmd = f"bash -c 'source {config_file} && docker compose -f docker-compose.yml -f docker-compose.mlnode.yml run --rm --entrypoint /bin/sh tmkms -c \"tmkms-pubkey\"'"
    
    pubkey_result = subprocess.run(
        pubkey_cmd,
        shell=True,
        cwd=working_dir,
        capture_output=True,
        text=True
    )
    
    print("Consensus key extraction completed!")
    print("Output:")
    print("=" * 50)
    if pubkey_result.stdout:
        print(pubkey_result.stdout)
    if pubkey_result.stderr:
        print("Errors/Warnings:")
        print(pubkey_result.stderr)
    print("=" * 50)
    
    # Extract consensus key from output
    full_output = pubkey_result.stdout + pubkey_result.stderr if pubkey_result.stderr else pubkey_result.stdout
    consensus_key_match = re.search(r'([A-Za-z0-9+/=]{40,})', full_output)
    if consensus_key_match:
        consensus_key = consensus_key_match.group(1)
        print(f"Extracted consensus key: {consensus_key}")
        # Store in CONFIG_ENV for potential future use
        CONFIG_ENV["CONSENSUS_KEY"] = consensus_key
    else:
        print("Warning: Could not extract consensus key from output")
        print("Full output for debugging:")
        print(full_output)
    
    if pubkey_result.returncode != 0:
        print(f"Consensus key extraction failed with return code: {pubkey_result.returncode}")
        raise subprocess.CalledProcessError(pubkey_result.returncode, pubkey_cmd)
    
    print("Consensus key extraction completed successfully!")


def main():
    if Path(os.getcwd()).absolute() != BASE_DIR:
        print(f"Changing directory to {BASE_DIR}")
        os.chdir(BASE_DIR)

    # Prepare
    clean_state()
    clone_repo()
    create_state_dirs()
    install_inferenced()

    # Create local 
    create_account_key()
    create_config_env_file()
    pull_images()
    run_genesis_initialization()
    extract_consensus_key()


if __name__ == "__main__":
    main()
