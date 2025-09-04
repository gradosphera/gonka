# Prerequisites:
# - Nebius CLI installed and configured with your credentials
# - jq installed for JSON parsing
# - Set project ID in nebuius CLI:
#   nebius config set parent-id project-e00pbskken10vwe2ptydhw

# Create a new network and subnet in Nebius cloud
NETWORK_NAME="testnet-network"
SUBNET_NAME="testnet-subnet"
export NB_NETWORK_ID=$(nebius vpc network create \
   --name "$NETWORK_NAME" \
   --format json | jq -r ".metadata.id")
export NB_SUBNET_ID=$(nebius vpc subnet create \
   --name "$SUBNET_NAME" \
   --network-id "$NB_NETWORK_ID" \
   --format json | jq -r ".metadata.id")

# Go to nebius AI cloud and create a L40S, 1CPU, 64GB RAM instance

# To connect to the instance:
ssh ubuntu@89.169.111.79

# Copy the script to the instance
scp genesis.py testnet@89.169.111.79:/home/testnet/

# Additional on-machine steps:
sudo apt get update

# Install huggingface-cli
python3 -m venv ~/py-venv
source py-venv/bin/activate
export HF_HOME="/home/ubuntu/hf-cache"
mkdir "$HF_HOME"
pip install -U "huggingface_hub[cli]"
huggingface-cli download Qwen/Qwen2.5-7B-Instruct
