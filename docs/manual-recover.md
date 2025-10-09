Earlier today, Gonka Chain experienced a temporary chain halt triggered by a deterministic panic.
- The incident did not cause a network partition, the state remains fully consistent
- More than two-thirds of the network has already recovered and is operating normally
- To resume block production, validators can perform a single-block rollback and upgrade to the patched binary

Proposed fix: https://github.com/gonka-ai/gonka/pull/384

Recovery Instructions  

1\ Stop the container (from gonka/deploy/join/ directory)
```
docker compose down node && sudo rm -rf .inference/data/upgrade-info.json
```

2\ Open the terminal of the container
```shell
source config.env && docker compose run -it node  /bin/sh
```

3\ Rollback gonka blockchain one block back
```
inferenced rollback --home /root/.inference/
```

4\ Exit the container terminal
```
exit
```

5\ Download the new version of the chain node with the patch
```shell
wget -O inferenced https://github.com/gonka-ai/gonka/releases/download/release%2Fv0.2.3-patch3/inferenced

sudo mkdir -p .inference/cosmovisor/upgrades/v0.2.3-patch3/bin/
sudo cp inferenced .inference/cosmovisor/upgrades/v0.2.3-patch3/bin/inferenced
sudo chmod +x .inference/cosmovisor/upgrades/v0.2.3-patch3/bin/inferenced
sudo rm .inference/cosmovisor/current
sudo ln -sf upgrades/v0.2.3-patch3 .inference/cosmovisor/current

echo "699b26ee2212146406ed4fe336428bfc134aade5cf03c55ba5c4f7ebf3ca6c90  .inference/cosmovisor/current/bin/inferenced" | sudo sha256sum -c --status - && echo SUCCESS || echo FAILED
```
You should get SUCCESS in return

6\ Start the updated node
```shell
source config.env && \
    docker compose up node api proxy -d --no-deps --force-recreate
```