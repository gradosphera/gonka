import os
import shutil
import hashlib
import urllib.request
from pathlib import Path


BASE_DIR = Path(os.environ["HOME"]).absolute()
GENESIS_VAL_NAME = "testnet-genesis"
GONKA_REPO_DIR = BASE_DIR / "gonka"


def clean_state():
    if GONKA_REPO_DIR.exists():
        print(f"Removing {GONKA_REPO_DIR}")
        shutil.rmtree(GONKA_REPO_DIR)
    
    if (BASE_DIR / "inferenced").exists():
        print(f"Removing {BASE_DIR / 'inferenced'}")
        shutil.rmtree(BASE_DIR / "inferenced")


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
    # Expected sha256: 24d4481bee27573b5a852265cf0672e1603e405ae1f1f9fba15a7a986feca569
    # download: https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.0/inferenced-linux-amd64.zip

    url = "https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.0/inferenced-linux-amd64.zip"
    expected_sha256 = "24d4481bee27573b5a852265cf0672e1603e405ae1f1f9fba15a7a986feca569"
    inferenced_zip = BASE_DIR / "inferenced-linux-amd64.zip"
    inferenced_path = BASE_DIR / "inferenced"
    
    # Download if not exists
    if not inferenced_zip.exists():
        print(f"Downloading {url}")
        urllib.request.urlretrieve(url, inferenced_zip)
    else:
        print(f"{inferenced_zip} already exists")
    
    # Verify checksum
    print(f"Verifying checksum...")
    with open(inferenced_zip, 'rb') as f:
        file_hash = hashlib.sha256(f.read()).hexdigest()
    
    if file_hash != expected_sha256:
        raise ValueError(f"Checksum mismatch! Expected: {expected_sha256}, Got: {file_hash}")
    else:
        print("Checksum verified successfully")
    
    # Extract if directory doesn't exist
    if not inferenced_path.exists():
        print(f"Extracting {inferenced_zip} to {inferenced_path}")
        os.system(f"unzip {inferenced_zip} -d {inferenced_path}")
    else:
        print(f"{inferenced_path} already exists")


def main():
    if Path(os.getcwd()).absolute() != BASE_DIR:
        print(f"Changing directory to {BASE_DIR}")
        os.chdir(BASE_DIR)

    # Prepare
    clean_state()
    clone_repo()
    create_state_dirs()
    install_inferenced()


if __name__ == "__main__":
    main()
