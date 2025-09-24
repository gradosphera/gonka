# Upgrade Proposal: v0.2.2

This document outlines the proposed changes for the first on-chain software upgrade. The `Changes` section details the major modifications, and the `Upgrade Plan` section describes the process for applying these changes.

## Upgrade Plan

This PR updates the code for the `api` and `node` services and modifies the container versions in `deploy/join/docker-compose.yml`.

The binary versions will be updated via an on-chain upgrade proposal. For more information on the upgrade process, please refer to `/docs/upgrades.md`.

Existing participants are **not** required to upgrade their `api` and `node` containers. The updated container versions are intended for new participants who join after the on-chain upgrade is complete.

**Proposed Process:**
1. Active participants review this proposal on GitHub.
2. Once the PR is approved by a majority, a `v0.2.2` release will be created from this branch, and an on-chain upgrade proposal for this version will be submitted.
3. If the on-chain proposal is approved, this PR will be merged immediately after the upgrade is executed on-chain.

Creating the release from this branch (instead of `main`) is intended to minimize the time that the `/deploy/join/` directory on the `main` branch contains container versions that do not match the on-chain binary versions, ensuring a smoother onboarding experience for new participants.


## Testing

### Testnet

The on-chain upgrade from version `v0.2.0` to `v0.2.2` has been successfully deployed and verified on the testnet.

We encourage all reviewers to request access to our testnet environment to validate the upgrade. Alternatively, reviewers can test the on-chain upgrade process on their own private testnets.

### Migration on real chain data

In addition to testnet validation, the migration logic is tested against a snapshot of mainnet data to ensure data integrity and a smooth transition. This procedure will be repeated immediately before the mainnet upgrade.


## Changes

### Exact token enforcement in Completion API (`3f49d869d9b4d70b0c0ac8d783fec65745978592`)

In the previous version, `api` nodes passed generated text in the `enforced_str` parameter during inference validation. In some cases, the tokenization of the generated text might produce a different token sequence than the one originally generated. This is a low-probability issue, but when it occurs during inference validation, it breaks the metric computation between artifacts. The network was protected from incorrect validation by reporting inferences with non-matching tokenization as always valid.

This commit switches to enforcing the exact sequence of tokens that was generated and the exact top-k candidates, and it enables strict validation of the matching sequence.

### Potential vulnerability in `POST /v1/participants` (`2927b241616ba08ae596dc1e029746f6c219c443`)

The `POST /v1/participants` API endpoint had a potential vulnerability. When it was called with empty `address` and `pubkey` fields, they were automatically filled with the node's owner account data.

Now, this process is simplified, and these fields are checked explicitly. A transaction is recorded only when the address does not yet exist on-chain.


### Prevent negative-coin panics in keeper flows (`f0ed9b331b2a60a88d34bdc04dc503f61e81ad67`)

This commit fixes possible panics caused by negative coin values. `sdk.NewInt64Coin` causes a panic if it receives a negative value. This change adds error checking for every instance where `NewInt64Coin` is used (except for a few in Genesis that can be ignored).
It also adds a safe method for logging subaccount transactions to reduce the required boilerplate and moves from using `uint64` in internal methods. In the future, `uint64` should be used strictly at the edges (messages, APIs) only. The risk of `0 - 1` resulting in `uint64.Max` is real, and Go idioms generally avoid using unsigned integers.


### Missed validations recovery before claim (`801299d4bac47e5267451bdbebb9f46c6bc4c3b1`)

#### Problem

During an epoch, a participant may legitimately miss validating some inferences due to network instability, hardware changes, or other temporary issues. Currently, once accounts are settled at an epoch transition, missed validations cannot be recovered. This leads to:

- Gaps in inference validation coverage.
- Potentially lower reputation and compute credit for participants.
- Inconsistent incentives between those who missed validations for legitimate reasons and those who didn’t.
- A risk that some invalid inferences remain undetected if they were missed by validators.

#### Solution

This change introduces a recovery mechanism that allows participants to "catch up" on missed validations after account settlement but before claiming their reward.

Now, before the actual claim, each participant:
- Queries for any inferences it should have validated but did not.
- Executes and submits any missed validations before submitting the claim transaction.

For such validations, participants receive **validation credit** (reputation / proof of compute) and still receive earned Bitcoin-style rewards. However, they do not receive a share of the work coin, since payment has already been settled.

If a late validation identifies an invalid inference:
- Invalidation verification and voting still occur on the network.
- If the inference is found to be invalid, the submitter's reputation is still penalized. However, the requester does not receive a refund, since funds have already been distributed and cannot be clawed back.

Further, the system might be improved by having participants perform regular validation recovery during the epoch and prohibiting validations after settlement entirely.


Additionally, this commit adds:
- Validation will now be retried several times before giving up, in an attempt to reduce the need for recovering missed validations.
- Proper filtering of models supported by a participant, to avoid attempting to validate models that are not supposed to be deployed on that participant's MLNode. This did not cause any errors previously but made the logic less clear.


### On-chain MLNode versioning and partial upgrades (`b2ef4fbe355431ad31d72b22aa062dce4892bfbd`)

This commit introduces a mechanism for automatic on-chain upgrades of MLNode containers. The upgrade is enabled by:

**Architecture Overview:**
```
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│ decentralized-  │───▶│  ML Proxy    │───▶│ MLNode v3.0.8   │
│ api             │    │  (NGINX)     │    │ (old version)   │
└─────────────────┘    │              │    └─────────────────┘
                       │              │    ┌─────────────────┐
                       │              │───▶│ MLNode v3.0.10  │
                       └──────────────┘    │ (new version)   │
                                           └─────────────────┘
```

#### Key Changes:

- MLNode version is now stored on-chain and managed by partial upgrades.
- The `api` service fetches the version at startup and updates automatically when an upgrade height is reached.
- The `api` stores the last-used MLNode version in its config. If the current version changes, it refreshes all MLNode clients and calls `.stop()` on the old version's clients.
- Fixed a bug where partial upgrades were ignored because the `Name` field was empty.
- Removed local version plan storage from the `api` config.
- Refactored and stabilized `testermint` integration tests to reduce flakiness.
- Adds an upgrade handler and migration for the on-chain upgrade.


### Dynamic Docker address resolution for proxy (`2845b898bd48b7203bd7f3302781faf4f160f900`)
- Dynamically resolves MLNode upstream in the NGINX template/entrypoint.
- Updates `deploy/join/nginx.conf` and proxy scripts for container networks.
- Improves robustness across host/container setups.

### Fix: Additional check for nonce duplicates on-chain (`b7d7600ec49ced4dfbedf6c01305aa5205f8d78`)

The previous version had a bug where identical nonces were taken into account when a node's weight was computed.
This fix ensures nonces are deduplicated before weights are assigned.

### Fix: Preventing generation of nonce duplicates (`1ae494372cb216046ab25ee179e3c21f392c5329`)

`Node.NodeNum` and `StartPoCNodeCommand.TotalNodes` fields are used to guarantee a unique nonce sequence for each participant's MLNode. The previous version used `len(nodesToDispatch)` as the value for `TotalNodes`.
At the same time, if a node is deleted and re-added in the same epoch, its `NodeNum` is determined by incrementing `b.curMaxNodesNum`. This can cause `NodeNum` to be greater than `len(nodesToDispatch)`, which could break the uniqueness of nonce sequences.

This commit fixes this by using `curMaxNodesNum` as `TotalNodes`.

### Proposals for new way to validation inference `dee011bfbc41f43cefde0c64b1efc6746976e01e`

### Paginator fixes `ecdfb135773f1e4854468a0e7585bd54bfe1d9fd`

This PR fixes a critical issue where queries intended to fetch all items were silently truncated to the first 100 results due to default Cosmos SDK pagination settings. This could lead to incomplete data processing and inconsistent state.

#### Key Changes

*   **`SettleAccounts`**: Updated to read all participants directly from the store, which is more appropriate and efficient for on-chain logic. Added tests to make sure we're settling for all the participants in the state and not only for the first 100.
*   **`get_participants_handler`**: Implemented manual per-page fetching for the public API endpoint. Queries are pinned to a specific block height to ensure data consistency across pages.
*   **`GetPartialUpgrades`**: Corrected to use a new `GetAllWithPagination` utility, ensuring all partial upgrade plans are fetched from the chain.


### Multiple Fixes and Security Improvements (`3ea3e5b5b1aa758e29741d5a5312dd41be10bf95`)

The commit introduces multiple fixes and security improvements:


#### Missed `node_id` to PoCBatch (`inference-chain/x/inference/keeper/msg_server_submit_poc_batch.go`)

`node_id` is used to detect which node produced the nonce's batch


#### Remove legacy weight distribution from batches without `node_id`

`inference-chain/x/inference/module/model_assignment.go` distributed before batches without `node_id` between another nodes. As now all MLNodes returns `node_id` => that it not needed anymore


#### Remove MLNodes without HardwareNodes or models supported by Governance

`unassignedMLNodes` from `inference-chain/x/inference/module/model_assignment.go` are not counted in total weight anymore
Total weight of participant is recomputed after all filtering 

#### Statistical Validation for Missed inference and validation

Binom test `inference-chain/x/inference/calculations/stats.go` is now used for:   
- not pay reward if statistically signigicant > 10% of requests are missed (`inference-chain/x/inference/keeper/accountsettle.go`) 
- not pay claim if statistically signigicant > 10% of validations are missed (`inference-chain/x/inference/keeper/msg_server_claim_rewards.go`) instead of hard check for all validations

#### Set Participant status to `ACTIVE` for all ActiveParticipant when switch epoch

`inference-chain/x/inference/module/module.go:moveUpcomingToEffectiveGroup`

#### Fix counter for successful inferences

`inference-chain/x/inference/keeper/msg_server_start_inference.go`  
`inference-chain/x/inference/keeper/msg_server_finish_inference.go`  

#### Fix for interrupted inference validation


#### Logging 


### Accept Claim only from CurrentEpoch -1 (`c8e6aefcf88958f5f2968cc9f305e1d6c0028dcd`)