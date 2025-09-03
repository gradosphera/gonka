import os
from pathlib import Path


BASE_DIR = Path(os.environ["HOME"]).absolute()
GENESIS_VAL_NAME = "testnet-genesis"
GONKA_REPO_DIR = BASE_DIR / "gonka"


def clean_state():
    GONKA_REPO_DIR.rmdir()


def clone_repo():
    os.system(f"git clone https://github.com/gonka-ai/gonka.git {GONKA_REPO_DIR}")


def create_state_dirs():
    template_dir = GONKA_REPO_DIR / "genesis/validators/template"
    my_dir = GONKA_REPO_DIR / f"genesis/validators/{GENESIS_VAL_NAME}"
    os.system(f"cp -r {template_dir} {my_dir}")


def install_inferenced():
    # clone sha256:24d4481bee27573b5a852265cf0672e1603e405ae1f1f9fba15a7a986feca569
    # download: https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.0/inferenced-linux-amd64.zip

    inferenced_zip = BASE_DIR / "inferenced-linux-amd64.zip"
    inferenced_zip.download_from_url("https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.0/inferenced-linux-amd64.zip")
    os.system(f"unzip {inferenced_zip} -d {BASE_DIR / "inferenced"}")

def main():
    if os.getcwd().absolute() != BASE_DIR:
        print(f"Changing directory to {BASE_DIR}")
        os.chdir(BASE_DIR)

    clean_state()
    clone_repo()
    create_state_dirs()


if __name__ == "__main__":
    main()
