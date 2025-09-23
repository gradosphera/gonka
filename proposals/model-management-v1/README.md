# Model Management Proposal

## Overview

This proposal describes the model management architecture in the decentralized inference system, focusing on how MLNodes track and support AI models throughout the system lifecycle. The architecture uses a three-tier approach: broker-based node tracking, on-chain hardware node storage, and epoch-based model snapshots for consistent inference routing.

## System Architecture

### 1. Broker Node Tracking

The **Broker** in `decentralized-api/broker/broker.go` serves as the central coordinator for MLNode operations. During startup, nodes are loaded through `LoadNodeToBroker` and registered via `RegisterNode` commands. Each node maintains a `Models` map containing supported AI model identifiers and their configuration arguments.

The broker continuously synchronizes with the blockchain through `syncNodes()`, which runs every 60 seconds via `nodeSyncWorker`. This process calculates differences between local node configurations and on-chain state using `calculateNodesDiff`, then submits updates through `SubmitHardwareDiff` messages.

### 2. On-Chain Hardware Node Storage

Hardware nodes are persisted on the blockchain through the **HardwareNodes** structure in `inference-chain/x/inference/keeper/hardware_node.go`. The `MsgSubmitHardwareDiff` handler in `msg_server_submit_hardware_diff.go` validates that all node models exist in the governance registry before accepting updates.

Each `HardwareNode` contains:
- `local_id`: Unique identifier within participant
- `status`: Current operational status (INFERENCE, POC, TRAINING, STOPPED, FAILED)  
- `models`: Array of supported model identifiers
- `hardware`: GPU/CPU specifications
- `host`/`port`: Network configuration

### 3. Epoch-Based Model Snapshots

During epoch transitions, the system creates snapshots of hardware node configurations through `setModelsForParticipants` in `inference-chain/x/inference/module/model_assignment.go`. These snapshots are stored in **EpochGroupData** structures, organized hierarchically with parent groups containing all models and subgroups for specific models.

The broker populates epoch-specific data in `NodeState.EpochModels` and `NodeState.EpochMLNodes` through `UpdateNodeWithEpochData`. This ensures consistent model assignments throughout an epoch, even if hardware configurations change mid-epoch.

## Key Questions and Scenarios

### 1. MLNodes Without Governance Model Support

**Question**: What happens if an MLNode doesn't support any governance-approved models? Do we accept PoC batches from such nodes?

**Current Behavior**: The system handles this scenario through a multi-layered approach:

- **Model Assignment Phase**: In `model_assignment.go`, nodes without governance model support are excluded from model-specific epoch subgroups. The `supportedModelsByNode` function filters nodes that don't match any governance models, and such nodes receive empty `supportedModels` arrays.

- **PoC Participation**: Nodes without governance models can still participate in PoC operations. The `postGeneratedBatches` handler in `decentralized-api/internal/server/mlnode/post_generated_batches_handler.go` accepts PoC batches regardless of model support. However, these nodes won't be assigned to inference subgroups.

- **Legacy Weight Distribution**: The `distributeLegacyWeight` function in `model_assignment.go` handles PoC batches submitted without NodeId (legacy behavior), distributing PoC weight among nodes that don't support governance models.

**Implications**: Non-governance nodes contribute to network security through PoC mining but cannot serve inference requests, creating a two-tier system where governance compliance is required for inference revenue but not for PoC participation.

### 2. Model Update Timing and Epoch Recording

**Question**: When a node updates its supported models, it will be recorded to the epoch only later. When will the broker change which model is requested?

**Current Behavior**: Model updates follow a delayed activation pattern:

- **Immediate Broker Update**: When nodes update their model configurations, the broker immediately reflects these changes in local `Node.Models` maps and submits them to the blockchain via `SubmitHardwareDiff`.

- **Blockchain Recording**: The chain immediately stores updated hardware configurations in `HardwareNodes`, but these changes don't affect active inference routing.

- **Epoch Activation**: Model changes only become effective during the next epoch transition when `setModelsForParticipants` creates new epoch snapshots. The broker continues using `NodeState.EpochModels` for inference routing until the next epoch.

- **Broker Model Selection**: The `InferenceUpNodeCommand` in `node_worker_commands.go` prioritizes epoch-assigned models over broker-configured models. Model request changes occur only when new epoch data is populated via `UpdateNodeWithEpochData`.

**Timeline**: Model updates are immediate for PoC participation but delayed for inference participation until the next epoch boundary, ensuring consistency during active inference periods.

### 3. MLNode Disabling and On-Chain Recording

**Question**: When we disable an MLNode, do we record it on-chain in hardware nodes?

**Current Behavior**: Node disabling is handled through multiple mechanisms:

- **Administrative Disabling**: Nodes can be disabled through the admin API using `NodeAdminCommand` in `node_admin_commands.go`. This sets `NodeState.AdminState.Enabled = false` and updates the epoch when disabling takes effect.

- **Status Propagation**: Disabled nodes have their `IntendedStatus` set to `HardwareNodeStatus_STOPPED`. The reconciliation process ensures the physical node transitions to stopped state.

- **On-Chain Recording**: The `syncNodes` function includes disabled nodes in hardware diffs with `status = STOPPED`. The `SubmitHardwareDiff` message updates the on-chain `HardwareNode.status` field, creating a permanent record of the node's disabled state.

- **Epoch Exclusion**: Disabled nodes are excluded from epoch group formation through `ShouldBeOperational` checks in `NodeState`, preventing them from receiving inference assignments or PoC tasks.

**Persistence**: Node disabling is recorded both locally in broker state and persistently on-chain through hardware node status updates, ensuring consistent exclusion from network operations.

### 4. Inference Execution with Inaccessible APIs

**Question**: What if executor APIs are not accessible? Do we still publish inference requests with unavailable APIs?

**Current Behavior**: The system publishes inference start requests regardless of executor API accessibility:

- `GetRandomExecutor` in `query_get_random_executor.go` selects participants based on epoch group membership and PoC slot availability, without checking API health
- The inference request is published to the blockchain via `StartInference` before attempting to contact the executor
- In `post_chat_handler.go`, if the executor API is unreachable, the inference fails but the on-chain request remains active
- Failed requests result in inference expiration through `handleExpiredInference()`, which issues refunds and increments the executor's `MissedRequests` counter
- No fallback mechanism exists - each inference request is bound to a single selected executor

### 5. Participant Punishment for Missed Inferences

**Question**: If a participant misses many inferences, how do we punish them?

**Current Behavior**: The system tracks missed inferences and applies financial penalties, but does NOT change participant status to `INVALID`:

- **Tracking**: Each expired inference increments `executor.CurrentEpochStats.MissedRequests` in `handleExpiredInference()`
- **Downtime Slashing**: At epoch end, `CheckAndSlashForDowntime()` calculates missed request percentage and slashes collateral if it exceeds `DowntimeMissedPercentageThreshold` (default 5%)
- **Slash Amount**: Participants lose `SlashFractionDowntime` (default 10%) of their total collateral for excessive downtime
- **No Status Change**: Missed inferences result in financial penalties but do NOT cause participants to be marked as `INVALID` - they remain `ACTIVE` and can continue participating in epochs
- **Separate System**: Invalid inference results (quality issues) are handled separately through validation and can lead to `INVALID` status via `calculateStatus()` function

### 6. Jailing and Exclusion Mechanisms

**Question**: If a participant misses many inferences sequentially, what are the consequences? Are they excluded from rewards and work?

**Current Behavior**: The system does NOT jail participants for missing inferences. Missing inferences only results in financial penalties:

- **No Inference Jailing**: Missing many inferences does NOT trigger jailing - participants face only financial penalties through collateral slashing
- **Validator Jailing Only**: `SetJailed()` is only called through staking hooks when validators are jailed for standard Cosmos issues (e.g., downtime, double-signing)
- **INVALID Status Instead**: Participants with quality issues (invalid inference results) are marked as `ParticipantStatus_INVALID`, which excludes them from rewards and work
- **Reward Exclusion**: `INVALID` participants are excluded from PoC weight calculations in `CalculateParticipantBitcoinRewards()` and receive zero rewards
- **No Automatic Recovery**: `INVALID` status persists until manually changed through governance or other mechanisms

### 7. On-Chain StartInference Without HTTP Request

**Question**: What happens if a transfer node posts StartInference on-chain but doesn't send the actual HTTP request to the executor? Will the executor listen for that event and react?

**Current Behavior**: Executors do NOT listen for StartInference events on-chain and only respond to HTTP requests:

- **No Event Listening**: Executors have no event handlers for StartInference events - they only listen for `inference_finished`, BLS events, and training events
- **HTTP-Only Execution**: Executors only process inferences when they receive HTTP requests at their `/v1/chat/completions` endpoint
- **Dual Publishing**: Transfer nodes publish both on-chain (`StartInference` message) and HTTP request simultaneously in `post_chat_handler.go`
- **Orphaned Requests**: If a transfer node posts StartInference but fails to send the HTTP request, the on-chain request will remain `STARTED` status but never be processed
- **Automatic Expiration**: Orphaned requests eventually expire through `handleExpiredInference()` after `ExpirationBlocks`, resulting in refunds and incrementing the assigned executor's `MissedRequests` counter
- **No Executor Penalty**: The executor is not at fault and receives a missed request penalty despite never receiving the actual work

## Implementation References

**Core Files**:
- `decentralized-api/broker/broker.go` - Central node coordination and synchronization
- `inference-chain/x/inference/keeper/msg_server_submit_hardware_diff.go` - Hardware update validation
- `inference-chain/x/inference/module/model_assignment.go` - Epoch-based model assignment
- `decentralized-api/broker/node_worker_commands.go` - Model loading and inference commands
- `inference-chain/proto/inference/inference/hardware_node.proto` - Hardware node data structures
- `inference-chain/proto/inference/inference/epoch_group_data.proto` - Epoch snapshot structures

**Key Functions**:
- `Broker.syncNodes()` - Hardware synchronization
- `setModelsForParticipants()` - Epoch model assignment  
- `UpdateNodeWithEpochData()` - Epoch data population
- `InferenceUpNodeCommand.Execute()` - Model loading for inference
- `ShouldBeOperational()` - Node operational status checks

## Required Implementation Changes

### 1. Remove Legacy PoC Distribution and Enforce Model Compliance

**Problem**: The current system has two issues: (1) `distributeLegacyWeight()` distributes PoC weight from legacy batches (submitted without NodeId) to all hardware nodes regardless of governance model support, and (2) MLNodes without any supported governance models still receive PoC weight and are included in epoch snapshots, allowing non-compliant nodes to participate in consensus.

**Solution**: Eliminate the legacy PoC distribution mechanism entirely by removing `distributeLegacyWeight()` calls and legacy MLNode handling. Implement governance model validation during epoch formation to exclude non-compliant MLNodes from PoC weight distribution and epoch assignments. Add epoch snapshot validation in `SubmitPocBatch()` to reject batches from nodes not included in current epoch formation.

**Changes Required**:

**File**: `inference-chain/x/inference/module/model_assignment.go`
- **Function**: `setModelsForParticipants()`
- **Current Logic**: Calls `distributeLegacyWeight()` to distribute PoC weight from legacy batches to all hardware nodes
- **New Logic**: 
  1. Remove the call to `distributeLegacyWeight(originalMLNodes, hardwareNodes, preservedNodes[p.Index])`
  2. Remove any legacy MLNodes (nodes with empty NodeId) from `originalMLNodes` before processing
  3. Filter remaining MLNodes using `supportedModelsByNode()` to exclude nodes without governance model support
  4. Log exclusions: `ma.LogInfo("Excluding MLNode without governance model support", "node_id", mlNode.NodeId)`
- **Result**: Only governance-compliant MLNodes with valid NodeIds proceed to epoch assignment

**File**: `inference-chain/x/inference/module/model_assignment.go`
- **Function**: `distributeLegacyWeight()`
- **Current Logic**: Distributes PoC weight from legacy batches to all hardware nodes
- **New Logic**: 
  1. **Remove this entire function** - legacy PoC distribution is no longer supported
  2. All PoC batches must be submitted with valid NodeId
  3. This eliminates the ability for nodes without governance model support to receive distributed PoC weight

**File**: `inference-chain/x/inference/module/model_assignment.go`
- **Function**: `supportedModelsByNode()`
- **Current Logic**: Returns map[nodeId][]modelId for all nodes
- **Enhancement**: Ensure function returns empty arrays `[]string{}` for nodes without any governance model matches
- **Validation**: Add explicit check that returned model arrays are non-empty before considering node as governance-compliant

**File**: `inference-chain/x/inference/keeper/msg_server_submit_poc_batch.go`
- **Function**: `SubmitPocBatch()`
- **Current Logic**: Accepts all PoC batches with valid `NodeId`
- **New Logic**: 
  1. **Require NodeId**: Reject any batch where `msg.NodeId` is empty (eliminates legacy batch support)
  2. After initial validation, query current epoch MLNode snapshots using `GetCurrentEpochGroupData()`
  3. Iterate through all subgroup `ValidationWeights` to find `msg.NodeId` in any `ml_nodes` array
  4. Return `types.ErrNodeNotInEpoch` if NodeId not found in current epoch snapshots
  5. This ensures only governance-compliant, epoch-assigned nodes can submit PoC batches

### 2. Model Alternative System

**Problem**: The current `IsValidGovernanceModel()` function in `msg_server_submit_hardware_diff.go` performs exact string matching between MLNode model identifiers and governance model IDs. This prevents model evolution scenarios where "Qwen/Qwen2.5-7B-Instruct" might be replaced by "Qwen/Qwen2.5-7B-Instruct-v2" without breaking existing MLNode configurations.

**Solution**: Extend the Model protobuf structure with an `alternative_models` array field, then implement a transitive model resolution system. Create a `ResolveModelId()` function that maps node model identifiers to canonical governance model IDs through alternative chains. Update all model validation points (`IsValidGovernanceModel()`, `supportedModelsByNode()`, `SubmitHardwareDiff()`) to use this resolution system, allowing nodes configured with alternative models to participate in governance model assignments.

**Changes Required**:

**File**: `inference-chain/proto/inference/inference/model.proto`
- **Message**: `Model`
- **Addition**: Add `repeated string alternative_models = 8;` field after existing fields
- **Regeneration**: Run `make proto-gen` to update `model.pb.go` with new field
- **Purpose**: Store array of model identifiers that can fulfill this governance model requirement

**File**: `inference-chain/x/inference/keeper/model.go`
- **Function**: `IsValidGovernanceModel(ctx context.Context, modelId string) bool`
- **Current Logic**: Simple lookup using `GetModel(ctx, modelId)`
- **New Logic**: 
  1. Direct match check: `GetModel(ctx, modelId)` 
  2. If not found, iterate through all governance models using `GetAllModels(ctx)`
  3. For each governance model, check if `modelId` exists in its `alternative_models` array
  4. Return true if found in any alternative array
- **Add Function**: `ResolveModelId(ctx context.Context, nodeModelId string) (string, bool)` - returns the canonical governance model ID that this node model can fulfill

**File**: `inference-chain/x/inference/module/model_assignment.go`
- **Function**: `supportedModelsByNode()`
- **Current Logic**: Checks `slices.Contains(node.Models, model.Id)` for exact matches
- **New Logic**: 
  1. Check direct match: `slices.Contains(node.Models, model.Id)`
  2. Check alternative matches: iterate through `node.Models` and call `ResolveModelId()` for each
  3. If any node model resolves to current governance `model.Id`, include in supported models
- **Result**: Returns map[nodeId][]governanceModelId where governance models include those fulfilled by alternatives

**File**: `inference-chain/x/inference/keeper/msg_server_submit_hardware_diff.go`
- **Function**: `SubmitHardwareDiff()`
- **Current Logic**: `if !k.IsValidGovernanceModel(ctx, modelId) { return nil, types.ErrInvalidModel }`
- **New Logic**: Replace with enhanced `IsValidGovernanceModel()` that includes alternative resolution
- **No interface change**: Existing validation logic continues to work, now with expanded model acceptance

**File**: `inference-chain/x/inference/keeper/model.go`
- **Add Function**: `GetModelAlternatives(ctx context.Context, governanceModelId string) []string`
- **Purpose**: Return all alternative model IDs for a given governance model
- **Usage**: Support debugging and administrative queries about model relationships

### 3. Comprehensive Node Disabling and On-Chain Admin State

**Problem**: The current system has inconsistent node disabling behavior. While `AdminState.Enabled = false` in the broker prevents local operations, disabled nodes might still be included in on-chain epoch formation if their status is not updated to `STOPPED` in time. The timing of this status update is also critical; it should not prevent a node from finishing its duties in the current epoch.

**Solution**: Implement a comprehensive node status management system by persisting the administrative state on-chain. This allows the system to proactively exclude nodes from upcoming work assignments before they are formally `STOPPED`. The transition to the `STOPPED` status will be delayed until the node has completed its obligations for the current epoch, specifically during the `ClaimReward` stage.

**Changes Required**:

**File**: `inference-chain/proto/inference/inference/hardware_node.proto`
- **Message**: `HardwareNode`
- **Addition**: Add a new field `AdminState admin_state = 8;` (assuming 8 is the next available field number).
- **New Message**: `AdminState`
  - `HardwareNodeStatus target_status = 1;` (e.g., `STOPPED`)
  - `int64 effective_epoch = 2;` (epoch when status should apply)
- **Regeneration**: Run `make proto-gen`.
- **Purpose**: To record the intended state of a node from an administrative action, and when that state should take effect.

**File**: `decentralized-api/broker/broker.go`
- **Function**: `syncNodes()`
- **Current Logic**: Submits all local nodes to chain via `calculateNodesDiff()` regardless of admin state, marking them for removal if disabled.
- **New Logic**:
  1. When a node is administratively disabled, `syncNodes` immediately submits a `HardwareDiff` to update the new `admin_state` field on-chain.
  2. The `target_status` would be set to `STOPPED` and `effective_epoch` to the next epoch.
  3. The node's main `status` field remains `INFERENCE` or its current operational state. This separates intended state from current state.

**File**: `inference-chain/x/inference/module/model_assignment.go`
- **Function**: `setModelsForParticipants()`
- **Current Logic**: The original proposal suggests filtering by `status`.
- **New Logic**:
  1. Before assigning work, filter out nodes based on their on-chain `admin_state`.
  2. Exclude any node where `admin_state.effective_epoch <= current_epoch`.
  3. This ensures nodes scheduled for disabling are not assigned new inference work for the upcoming epoch.
  4. Log exclusions: `ma.LogInfo("Excluding node with pending admin state from new epoch assignments", "node_id", node.LocalId, "effective_epoch", node.AdminState.EffectiveEpoch)`

**File**: `inference-chain/x/inference/module/end_blocker.go` (or similar epoch transition logic)
- **New Logic**:
  1. Add a new step that runs during the `ClaimReward` phase of the epoch.
  2. This step iterates through all `HardwareNode`s.
  3. If `current_epoch >= node.AdminState.effective_epoch` and `node.status != node.AdminState.target_status`, update `node.status` to `node.AdminState.target_status` (e.g., `STOPPED`).
  4. This correctly times the final state transition after the node has fulfilled its duties for the epoch it was active in.

**File**: `inference-chain/x/inference/keeper/msg_server_submit_poc_batch.go`
- **Function**: `SubmitPocBatch()`
- **Enhancement**: After validating NodeId exists in epoch snapshots (from requirement #1), also verify the corresponding hardware node `status` is not `STOPPED` or `FAILED`.
- **Error**: Return `types.ErrNodeDisabled` for batches from disabled nodes.

**Implementation Priority**:
1. **PoC Batch Validation** - Immediate security concern requiring strict enforcement
2. **Node Disabling** - Consistency issue affecting network participation
3. **Model Alternative System** - Feature enhancement for operational flexibility

**Validation Requirements**:
- All changes must maintain backward compatibility during transition periods
- Model alternative chains must prevent circular references
- Disabled node exclusion must be atomic across all network operations

### 4. Enhanced Punishment for Missing Inferences

**Problem**: Participants who miss inferences still receive full rewards during claim settlement. The current reward distribution in `GetSettleAmounts()` doesn't account for missed inference performance, allowing participants to collect full rewards despite poor service availability.

**Solution**: Implement statistical analysis for missed inferences during reward settlement and add sequential tracking with jailing for persistent failures.

**Changes Required**:

**File**: `inference-chain/x/inference/keeper/accountsettle.go`
- **Function**: `GetSettleAmounts()`
- **Current Logic**: Calculates rewards without considering missed inference statistics
- **New Logic**: 
  1. Before reward calculation, calculate participant's missed inference percentage
  2. Apply graduated penalties based on three-tier threshold system:
     - Minor (2.0%): Warning only, no penalty
     - Moderate (3.5%): 25% reward reduction
     - Major (5.0%): Full collateral slashing (existing) + zero rewards
  3. Log penalties applied based on threshold exceeded
- **Add Parameters**: Three-tier threshold system:
  - `MissedInferenceMinorThreshold` (default: 2.0% - warning level)
  - `MissedInferenceModeratethreshold` (default: 3.5% - partial penalty)  
  - `MissedInferenceMajorThreshold` (default: 5.0% - matches current collateral slashing threshold)

**File**: `inference-chain/proto/inference/inference/participant.proto`
- **Message**: `Participant`
- **Addition**: Add fields for sequential missed inference tracking:
  - `int64 last_missed_inference_block = 15;` - Block height of last missed inference
  - `int64 sequential_missed_inferences = 16;` - Counter for consecutive missed inferences
- **Regeneration**: Run `make proto-gen` to update participant structures

**File**: `inference-chain/x/inference/module/module.go`
- **Function**: `handleExpiredInference()`
- **Current Logic**: Only increments `MissedRequests` counter
- **New Logic**: 
  1. After incrementing `MissedRequests`, check if current block height differs from `LastMissedInferenceBlock`
  2. If different block, increment `SequentialMissedInferences` and update `LastMissedInferenceBlock`
  3. If `SequentialMissedInferences >= SequentialMissedInferenceThreshold` (default: 50), call staking module to jail the participant's validator
  4. Reset counter on successful inference completion

### 5. Exclude PoC Weight from Jailed Participants

**Problem**: Jailed participants continue to receive PoC weight and participate in consensus despite being penalized by the staking module.

**Solution**: Add jailed status checks during PoC weight calculations and epoch formation.

**Changes Required**:

**File**: `inference-chain/x/inference/keeper/bitcoin_rewards.go`
- **Function**: `GetParticipantPoCWeight()`
- **Current Logic**: Calculates PoC weight without checking jailed status
- **New Logic**: 
  1. Before calculating PoC weight, check if participant is jailed using `collateralKeeper.IsJailed(ctx, participantAddress)`
  2. Return 0 weight if participant is jailed
  3. Log exclusions: `k.LogInfo("Excluding jailed participant from PoC weight", "participant", participant)`

**File**: `inference-chain/x/inference/module/model_assignment.go`
- **Function**: `setModelsForParticipants()`
- **Current Logic**: Processes all active participants without jailed status checks
- **New Logic**: 
  1. After retrieving active participants, filter out jailed participants
  2. Use `collateralKeeper.IsJailed()` to check each participant's jailed status
  3. Exclude jailed participants from all epoch formation processes

### 6. Punishment for Missing Validations During Reward Claims

**Problem**: The current system already prevents participants from claiming rewards if they miss validations, but uses different thresholds than inference penalties and doesn't provide graduated consequences.

**Solution**: Implement separate validation penalty system using the same three-tier thresholds as inference penalties, since missed inference stats are cleared after epoch settlement and cannot be compared.

**Changes Required**:

**File**: `inference-chain/x/inference/keeper/msg_server_claim_rewards.go`
- **Function**: `ClaimRewards()`
- **Current Logic**: Binary reward exclusion for missed validations
- **New Logic**: 
  1. Calculate participant's missed validation percentage for the current epoch
  2. Apply graduated penalties using same three-tier threshold system as inference penalties
  3. Apply penalties to current reward claim: Minor (warning), Moderate (25% reduction), Major (50% reduction)
  4. This operates independently from inference penalties since inference stats are cleared after settlement
- **Add Parameters**: Use same three-tier threshold system as inference penalties

### 7. API Node StartInference Event Listening and Orphaned Request Handling

**Problem**: Executors can be unfairly penalized when transfer nodes post StartInference on-chain but fail to send HTTP requests, creating orphaned requests that expire without executor knowledge.

**Solution**: Implement StartInference event listening in API nodes to detect orphaned requests and automatically process them.

**Changes Required**:

**File**: `decentralized-api/internal/event_listener/event_listener.go`
- **Function**: `NewEventListener()`
- **Addition**: Add new event handler for StartInference events
- **New Handler**: `StartInferenceEventHandler` to process on-chain inference requests

**File**: `decentralized-api/internal/event_listener/event_listener.go`
- **Add Handler**: `StartInferenceEventHandler`
- **Implementation**: Create new event handler that tracks on-chain requests and compares with received HTTP requests, automatically processing inferences if no HTTP request received within timeout

**File**: `decentralized-api/internal/event_listener/inference_tracker.go`
- **New File**: Create in-memory tracking system for inference requests
- **Structure**: Track both on-chain requests and HTTP requests with timestamps
- **Functions**:
  - `TrackOnChainRequest()` - Record StartInference events
  - `TrackHTTPRequest()` - Record received HTTP requests
  - `CheckOrphanedRequests()` - Identify requests without corresponding HTTP calls
  - `ProcessOrphanedRequest()` - Automatically handle orphaned inferences

**File**: `decentralized-api/internal/server/public/post_chat_handler.go`
- **Function**: `handleTransferRequest()`
- **Addition**: After processing HTTP request, mark it as received in `InferenceRequestTracker`
- **New Logic**: Call `tracker.TrackHTTPRequest(inferenceUUID)` to record HTTP request receipt

**Implementation Priority**:
1. **Sequential Missed Inference Tracking** - Critical for preventing persistent bad actors
2. **Statistical Punishment Systems** - Important for fair resource allocation
3. **Jailed Participant Exclusion** - Consistency with Cosmos staking principles
4. **StartInference Event Listening** - Fairness improvement for executors

**New Parameters Required**:
- **Three-Tier Threshold System** (applies to both inferences and validations):
  - `MinorThreshold` (default: 2.0% - warning level)
  - `ModerateThreshold` (default: 3.5% - partial penalty)
  - `MajorThreshold` (default: 5.0% - matches current collateral slashing, full penalty)
- `SequentialMissedInferenceThreshold` (default: 50)
- `OrphanedRequestTimeout` (default: 30 seconds)

### 8. Handling Validation Debt for Offline Nodes

**Problem**: A participant might turn off their nodes, making them unable to fulfill their inference validation responsibilities. This can lead to an accumulation of "validation debt" that is not currently accounted for, especially if the node is disabled before penalties for missed validations can be applied. This is a rare edge case but could affect network integrity.

**Solution**: Introduce a mechanism to track and penalize outstanding validation debt. This ensures that participants cannot evade their responsibilities by simply going offline.

**Changes Required**:

**File**: `inference-chain/proto/inference/inference/participant.proto`
- **Message**: `Participant`
- **Addition**: Add a field `int64 validation_debt = 17;` to track the number of missed validations that have not yet been penalized.

**File**: `inference-chain/x/inference/keeper/ slashing.go` (or a relevant keeper)
- **Logic**: When a participant misses a validation, increment their `validation_debt` counter.
- **New Function**: `SettleValidationDebt()`
  - This function would be called when a participant's node is being disabled or at other key lifecycle events.
  - It would apply a slash to the participant's collateral proportional to the accumulated `validation_debt`.
  - Once the penalty is applied, the `validation_debt` is reset to zero.

**File**: `inference-chain/x/inference/keeper/msg_server_submit_hardware_diff.go`
- **Function**: `SubmitHardwareDiff()`
- **Enhancement**: When a request is made to re-enable a node (e.g., move from `STOPPED` to `INFERENCE`), check if the participant has any outstanding `validation_debt`.
- **New Logic**: Reject the re-enabling request until the validation debt is settled, potentially through a manual transaction by the participant to clear it.
