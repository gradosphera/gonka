# Prepare and Test an Upgrade Proposal

This document describes the process of preparing and reviewing an upgrade proposal on the Gonka chain. There are different types of upgrades:

- full automatic upgrade of `api` and `node` binaries (on-chain, might break consensus)
- partial upgrade of `api` or `node` binaries (on-chain, might break consensus)
- semi-automatic upgrade of `mlnode` containers which include an automatic on-chain switch of version (on-chain, might break consensus)
- off-chain upgrades of any containers (off-chain, doesn't break consensus, such an upgrade can be done asynchronously)

This document focuses only on full on-chain upgrades. For more details on the overall upgrade strategy using Cosmovisor, see [the upgrade strategy document](./upgrades.md).

## 1. Process

The process consists of the following steps:

- create a PR in the `gonka-ai` repo named "Upgrade Proposal vX.Y.Z"
- get approval from a majority of active miners, and apply all requested edits
- once the PR is approved, build and release the upgrade binaries and containers with version tag `vX.Y.Z` from the PR branch. **Do not merge the PR yet.**
  - Run `make build-for-upgrade` from the repository root to build both `inferenced` and `decentralized-api` binaries
  - Binaries are published to `public-html/v2/inferenced/` and `public-html/v2/dapi/` with checksums automatically generated
  - Build and push Docker containers for all services with the version tag
  - Create a GitHub release (example: https://github.com/gonka-ai/gonka/releases/tag/release%2Fv0.2.3)
- submit the on-chain upgrade proposal with `./inferenced tx upgrade software-upgrade vX.Y.Z` and include links to the released binaries in the upgrade info JSON. The proposed upgrade height should be at least a couple of hours from the PoC phase. The governance proposal requires a deposit. See [the upgrade strategy document](./upgrades.md) for a full command example.
- attach the static GitHub link to `proposals/governance-artifacts/update-vX.Y.Z/README.md` as metadata to the on-chain proposal using `--metadata "https://github.com/gonka-ai/gonka/blob/<commit-hash>/proposals/governance-artifacts/update-vX.Y.Z/README.md"`
- get votes from the majority of miners
- if voting finishes and there is approval by consensus, the upgrade is applied automatically
- after a successful on-chain upgrade, merge the PR. This ensures the `main` branch is not in an inconsistent state where container versions do not match the on-chain binary versions.


Each upgrade proposal should start with a PR named "Upgrade Proposal vX.Y.Z". The PR should contain:

- all proposed changes
- incremented `ConsensusVersion` in each modified module's `module.go` file
- migrations for all incremented versions registered in `inference-chain/app/upgrades.go:registerMigrations`. If no migrations are needed, an empty migration should still be created to track that the module version has been processed
- an upgrade handler created in `inference-chain/app/upgrades/vX_Y_Z` and registered in `inference-chain/app/upgrades.go:setupUpgradeHandlers`
- container image versions in `deploy/join/docker-compose.yml` set to `X.Y.Z`
- a description of the proposal in `proposals/governance-artifacts/update-vX.Y.Z/README.md` and in the PR description


If an upgrade proposal has multiple independent features, they should be split into different commits. For clarity, structure the PR description with clear sections. Here is a recommended structure:

> ## Example PR Description
>
> ### Upgrade Plan
>
> This section should describe:
> - Which components are part of the on-chain upgrade and which can be updated off-chain asynchronously.
> - Any manual steps required for participants. It should be very clear if existing participants need to take action or if changes only affect new participants.
>
> ### Testing
>
> This section should describe:
> - How this upgrade proposal was tested and how it can be reproduced.
> - A confirmation that all checks from the `Testing at TestNet` section below have passed successfully.
>
> ### Risks
>
> - Any risks or changes in behavior after this upgrade should be documented here. If there are no known risks, this should be stated explicitly.
>
> ### Changes
>
> This section should provide:
> - A high-level summary of what the upgrade introduces.
> - A detailed description for every commit that introduces major changes, ideally with commit hashes. For example:
>
> #### Feature X (`<hash>`)
>
> A brief description of the feature and the changes it introduces.

Upgrade proposals must not require changes to the base container. Binaries downloaded by `cosmovisor` must have all required files.


## 2. Testing

Before review starts, the upgrade proposal should pass the following checks:

- All Unit Tests pass (`make local-build` locally or `.github/workflows/verify.yml` in CI/CD)
- All Integration Tests pass (`.github/workflows/integration.yml` in CI/CD or `make run-tests` locally)
- The upgrade process from the current Gonka Chain version has been tested successfully multiple times on TestNet
- The migration process is tested on a fresh export of Gonka MainNet state in a test environment


### Testing at TestNet

The TestNet is described in `test-net-cloud/nebius/README.md` and can be quickly redeployed in case of failures. All cold keys are available directly on TestNet's servers.

The process of proposing and voting on TestNet exactly matches the process on MainNet.

As TestNet might be re-created quite often, each test should be started by executing a couple of hundred inference requests to make testing closer to the real chain state.

Then the process is:

- schedule an upgrade in the middle of an epoch
- vote for the upgrade
- check that the upgrade is applied successfully and the new binaries are used
- check that the chain successfully serves and validates inference after the upgrade
- check that all active participants from the epoch with the upgrade were rewarded for this epoch
- check that the next PoC phase after the upgrade was successful and all active participants successfully passed it
- check that the testnet works without any problem for several next epochs after the upgrade and every node is rewarded