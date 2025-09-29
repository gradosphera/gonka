
# Claim Recovery

Current version are not able to claim reward under the high load. Having the new version, we can try to re-claim.


## Step 1: Check binary hashes

Check API binary:

# Check API binary
echo "43c2ebe46d5f5a1f336926397a562b67df02821920e4e434d136124a5757f83b .dapi/cosmovisor/current/bin/decentralized-api" | sudo sha256sum -c --status - && echo "API: UPGRADE NOT NEEDED" || echo "API: UPGRADE NEEDED"
```

If it**UPGRADE NEEDED**, it might be worth to check recovery.

## Step 2: Stop container

```bash
source config.env && docker compose down api
```


## Step 2: Recover API binary (if failed)

```bash
# Download and verify the API binary
wget https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.3-manual/decentralized-api-amd64.zip
echo "9239001463b4aa0bb664779fbd28f62c914b059124b51e9387ad915276223a3c decentralized-api-amd64.zip" | sha256sum -c --status - && echo SUCCESS || echo FAILED

# Install and symlink
sudo rm -rf .dapi/data/upgrade-info.json

sudo rm -rf .dapi/cosmovisor/upgrades/v0.2.3-patch/bin/
sudo mkdir -p .dapi/cosmovisor/upgrades/v0.2.3-patch/bin/
unzip decentralized-api-amd64.zip -d temp-api/
sudo cp temp-api/decentralized-api .dapi/cosmovisor/upgrades/v0.2.3-patch/bin/
sudo chmod +x .dapi/cosmovisor/upgrades/v0.2.3-patch/bin/decentralized-api
sudo rm .dapi/cosmovisor/current
sudo ln -sf upgrades/v0.2.3-patch .dapi/cosmovisor/current
rm -rf temp-api/ decentralized-api-amd64.zip
```

## Step 3: Verify API binary recovery

```bash
# Check API binary hash
echo "43c2ebe46d5f5a1f336926397a562b67df02821920e4e434d136124a5757f83b .dapi/cosmovisor/current/bin/decentralized-api" | sudo sha256sum -c --status - && echo "API: UPGRADE NOT NEEDED" || echo "API: UPGRADE NEEDED"
```

## Step 4: Restart container

```bash
source config.env && docker compose up -d api --force-recreate
```