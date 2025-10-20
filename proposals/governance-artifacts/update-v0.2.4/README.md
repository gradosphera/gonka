# Upgrade Proposal: v0.2.4

This document outlines the proposed changes for on-chain software upgrade v0.2.4. The `Changes` section details the major modifications, and the `Upgrade Plan` section describes the process for applying these changes.

## Upgrade Plan

This PR updates the code for the `api` and `node` services and modifies the container versions in `deploy/join/docker-compose.yml`.

The binary versions will be updated via an on-chain upgrade proposal. For more information on the upgrade process, refer to `/docs/upgrades.md`.

Existing participants are **not** required to upgrade their `api` and `node` containers. The updated container versions are intended for new participants who join after the on-chain upgrade is complete.

**Proposed Process:**
1. Active participants review this proposal on GitHub.
2. Once the PR is approved by a majority, a `v0.2.4` release will be created from this branch, and an on-chain upgrade proposal for this version will be submitted.
3. If the on-chain proposal is approved, this PR will be merged immediately after the upgrade is executed on-chain.

Creating the release from this branch (instead of `main`) minimizes the time that the `/deploy/join/` directory on the `main` branch contains container versions that do not match the on-chain binary versions, ensuring a smoother onboarding experience for new participants.

New MLNode container `v3.0.10` is fully compartible with `v3.0.9` and can be updated asyncronously at any time.  
**Important:** to support new features as models auto-downloading, `HF_HUB_OFFLINE` should be disabled (env variable removed from `deploy/join/docker-compose.mlnode.yml`).


## Testing

### Testnet

The on-chain upgrade from version `v0.2.3-patch2` to `v0.2.4`  has been successfully deployed and verified on the testnet.

We encourage all reviewers to request access to our testnet environment to validate the upgrade. Alternatively, reviewers can test the on-chain upgrade process on their own private testnets.

## Changes
---
### Training Security
See the commits related to this change [here](https://github.com/gonka-ai/gonka/pull/385/commits/c8a9391b29a7a21100f42a71f0ccd67f40bf0e05) and [here](https://github.com/gonka-ai/gonka/pull/358/commits/4d3e49de4aa499dee042c397062c78a0fc94413a)
#### Overview
Training in Gonka is not ready for broad use. However, all training messages are currently available, causing both confusion and an attack surface for DoS attachs
#### Solution
All training related messages are now behind two allow lists (EXEC to run training tasks, START to initiate training). The lists are controlled via governance votes.

This also addresses several Certik audit issues.

---
### Major Certik Audit fixes
This is a set of fixes that address issues found in the Certik Audit that required more substantial fixes in the code. References to the specific Certik issue are in parenthesis
The overall changes can be seen [here](https://github.com/gonka-ai/gonka/pull/385/commits/be00dc0188fd028a967b4167ed287af6124f4d14)
#### Pubkey must match address
When adding a new Participant, the pubkey must match the address (GOC-12). Specific changes [here](https://github.com/gonka-ai/gonka/pull/358/commits/1867daadd77c73f2807b273f2b2ce3dfa0107859)
#### Panic avoidance during EndBlock
Panics during EndBlock will cause _consensus failure_. These are a set of changes to avoid possible panics, either explicitly called or through methods starting with `Must`.  (GOC-13, GOI-24)
Changes are [here](https://github.com/gonka-ai/gonka/pull/358/commits/505a3e4be8f4d7489a95a5a7f9d05879095b1161) and [here](https://github.com/gonka-ai/gonka/pull/358/commits/eef739c614f8167b100819e5617253e53ac8d6df)
#### Validate inference timestamps on chain
While replay attacks are primarily avoided via Signature dedupe for InferenceId, after pruning older inferences could be replayed. This adds on-chain checks for old inferences being replayed. (GOI-01). Changes are [here](https://github.com/gonka-ai/gonka/pull/358/commits/fe2d5284c00e7e095b7a16fff00ccea4794d429e)
#### Pruning Improvements
The first version of pruning was crude and liable to issues as scale increased as all pruning would take place in a single block. This introduces a more scalable version of pruning that will prune a given number of items each block until all items are pruned, and introduces a generic `Pruner` that can be re-used for this logic for other items as needed. (GOC-14). Changes are [here](https://github.com/gonka-ai/gonka/pull/358/commits/1bbda08851a69f07b99efc5f6a7aa35d49171914) and [here](https://github.com/gonka-ai/gonka/pull/358/commits/f190ba597c819c213d291708a164e6a18f7fb15f)
#### Validation Limits
`MsgValidate` had zero limits, allowing even non-participants to submit invalidations, requiring and expensive chain-wide revalidation each time. Even inadvertent invalidations could cause significant strain on the chain, and have caused one chain halt.

There are several fixes to address this:
1. No longer include the `ResponsePayload` in `MsgValidate` (this was 90%+ of the message size)
2. Check that a validator is an active participant AND has the model for the Inference being validated
3. Add limits to the number of Invalidations allowed for each Participant (based on Power and Reputation)

Code is [here](https://github.com/gonka-ai/gonka/pull/358/commits/393ad481868ff04fb942a0a322c22eef560a304a) and [here](https://github.com/gonka-ai/gonka/pull/358/commits/ff68328ec21f692ac70415fd1ab8fab16baef771)

---
### Config Management Improvements for API nodes
#### Config Storage
Config Management was entirely file based for API nodes. While this was adequate for infrequently or never changing attributes such as URLs, account addresses or network settings, it is not performant or safe for frequently changing values such as block height, node data or seed info.
#### Solution
Introduce a file based SqlLite DB to handle changing values and synchronize them for the API node. 

Source code is [here](https://github.com/gonka-ai/gonka/pull/385/files/be00dc0188fd028a967b4167ed287af6124f4d14..7fcd6a374d0c2da565d6174a8eb5940acbc97957)

#### MLNode management api

REST API is becoming new main way to manage MLNodes. `node-config.json` is used only at the first load. 

New endpoint `PUT /admin/nodes/:id` is introduce to update MLNode infor without deleting.

---
### Unordered Transaction Timeout fix
#### Context
We use unordered transactions. Each transaction has a TTL—the time window within which the transaction must be executed. Some transactions are sent with a retry: we send repeatedly until we confirm that the transaction has made it to the chain
#### Problem
Under heavy load or other extreme circumstances, the chain blocks can begin to slow or even halt. Since transactions are compared with _block time_ for TTL and signed with _node machine time_, the drift would result in all messages being rejected as being ahead of block time. They are then resent, further propagating the error and resulting in _additional_ strain on the network as it tries to recover.
#### Solution
 - Use the latest block time instead of node machine time to sign and verify TTL for transactions
 - Detect slow or halted chains and stop sending retries to allow better chain recovery
 - Cap number of retry attempts (at 100 for now)

Source code is [here](https://github.com/gonka-ai/gonka/pull/385/commits/c17e288333c41d64ca9f38cf3c4873ed0a2f9e2d)

---
### Support small GPUs on TestNet
#### Problem
`testnet` should support smaller GPUs in order to encourage bigger participation in testnet, increasing tests and therefore network robustness.
#### Solution
- Add testenv specific environment variables to detect when the chain is testnet vs mainnet
- Add testenv specific params to have different proof-of-compute configurations (enabling smaller GPUs and faster testing)
Full changes are [here](https://github.com/gonka-ai/gonka/pull/385/commits/45fda6d3329f3b795e8a0c1bd86396665d39a3de)

---
### MLNode Management APIs & vLLM Stability Improvements
Far more details available in the PR description at the top [here](https://github.com/gonka-ai/gonka/pull/385/commits/492164a07d845b6dfb2b30a8a575ed7e5d46ddf1), but a summary:
1. Add new Endpoints to the ML Node that allow checking status of model downloads and other properties
2. Add new Endpoints to the ML Node that checking GPU status and drivers
3. Improvements in vLLM Stability deployment, as well as additional status APIs to check on progress
4. Integration of the above with the API node, with background pre-downloading of upcoming epochs and updating hardware on the chain.

---
### Health endpoints for MLNode and experimental auto-detection for node setup issues [here](https://github.com/gonka-ai/gonka/pull/385/commits/b7b284f42145dc0130a89fe151a74a13d2fe2571)