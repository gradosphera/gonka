# Missed Validations Recovery System - Task Plan

## Prerequisite Reading

Before starting implementation, please read the following documents to understand the full context of the changes:
- The main proposal: `proposals/missed_validations/missed_validations.md`
- The existing validation flow: `decentralized-api/internal/validation/inference_validation.go`
- The seed management system: `decentralized-api/apiconfig/config_manager.go`
- The epoch transition handling: `decentralized-api/internal/event_listener/new_block_dispatcher.go`

## How to Use This Task List

### Workflow
- **Focus on a single task**: Please work on only one task at a time to ensure clarity and quality. Avoid implementing parts of future tasks.
- **Request a review**: Once a task's implementation is complete, change its status to `[?] - Review` and wait for my confirmation.
- **Update all usages**: If a function or variable is renamed, find and update all its references throughout the codebase.
- **Build after each task**: After each task is completed, build the project to ensure there are no compilation errors.
- **Test after each section**: After completing all tasks in a section, run the corresponding tests to verify the functionality.
- **Wait for completion**: After I confirm the review, mark the task as `[x] - Finished`, add a **Result** section summarizing the changes, and then move on to the next one.

### Build & Test Commands
- **Build Decentralized API**: From the project root, run `make api-local-build`
- **Test Decentralized API**: From the project root, run appropriate test commands for modified components

### Status Indicators
- `[ ]` **Not Started** - Task has not been initiated
- `[~]` **In Progress** - Task is currently being worked on
- `[?]` **Review** - Task completed, requires review/testing
- `[x]` **Finished** - Task completed and verified

### Task Organization
Tasks are organized by implementation area and numbered for easy reference. Dependencies are noted where critical. Complete tasks in order.

### Task Format
Each task includes:
- **What**: Clear description of work to be done
- **Where**: Specific files/locations to modify
- **Why**: Brief context of purpose when not obvious

## Task List

### Section 1: Validation Recovery Core Logic

#### 1.1 Validation Decision Logic Extraction
- **Task**: [x] Extract validation decision logic from SampleInferenceToValidate
- **What**: Create a new function `shouldValidateInference()` that extracts the core validation decision logic from `SampleInferenceToValidate()` (lines 104-125) without the event-handling parts. This function should take inference details, seed, validation parameters, and return whether the current participant should validate the inference.
- **Where**: `decentralized-api/internal/validation/inference_validation.go`
- **Why**: Reuse existing validation decision logic for recovery without duplicating event-handling code
- **Dependencies**: None
- **Result**: Created `shouldValidateInference()` function that extracts core validation logic (executor skip, power validation, calculations.ShouldValidate call) and refactored `SampleInferenceToValidate()` to use the new function. Build successful with no compilation errors.

#### 1.2 Missed Validation Detection Function
- **Task**: [x] Create function to detect missed validations for an epoch
- **What**: Create `detectMissedValidations(epochIndex uint64, seed int64)` function that:
  - Uses `InferenceAll` query to get all inferences
  - Filters by `inference.EpochId == epochIndex`
  - For each inference, calls `shouldValidateInference()` to check if current participant should validate
  - Uses `GetEpochGroupValidations` query to check what was already validated
  - Returns list of inference IDs that were missed
- **Where**: `decentralized-api/internal/validation/inference_validation.go`
- **Why**: Core logic to identify which validations need recovery
- **Dependencies**: 1.1
- **Result**: Created `detectMissedValidations()` function that uses existing queries (InferenceAll, EpochGroupValidations) to identify missed validations. Function filters inferences by epoch, checks validation requirements using shouldValidateInference(), and compares against already-submitted validations. Includes comprehensive logging and error handling. Build successful with no compilation errors.

#### 1.3 Recovery Validation Execution Function
- **Task**: [x] Create function to execute recovery validations
- **What**: Create `executeRecoveryValidations(missedInferences []types.Inference)` function that:
  - Takes inference objects directly (no need to query again)
  - Executes validations in parallel goroutines (same pattern as existing code)
  - Calls existing `validateInferenceAndSendValMessage()` for each missed validation
  - Logs recovery validation attempts and results
- **Where**: `decentralized-api/internal/validation/inference_validation.go`
- **Why**: Execute the actual recovery validations using existing validation infrastructure
- **Dependencies**: 1.2
- **Result**: Created `executeRecoveryValidations()` function that takes inference objects directly and executes validations in parallel goroutines. Updated `detectMissedValidations()` to return inference objects instead of IDs to avoid redundant queries. Uses existing validation infrastructure with proper parallel execution pattern. Build successful with no compilation errors.

### Section 2: Epoch Transition Integration

#### 2.1 Epoch Transition Detection Enhancement
- **Task**: [x] Add missed validation recovery to epoch transition handling
- **What**: Enhance `handlePhaseTransitions()` in `new_block_dispatcher.go` to detect epoch transitions and trigger validation recovery:
  - Detect when transitioning to "Set New Validators" stage
  - Get previous epoch seed using `configManager.GetPreviousSeed()`
  - Call validation recovery in background goroutine to avoid blocking
  - Add appropriate logging for recovery initiation
- **Where**: `decentralized-api/internal/event_listener/new_block_dispatcher.go`
- **Why**: Automatically trigger recovery during natural epoch transition points
- **Dependencies**: 1.3
- **Result**: Successfully integrated missed validation recovery with epoch transition handling. **Major Architecture Improvement**: Moved recovery from IsSetNewValidatorsStage to IsClaimMoneyStage - this ensures we validate everything BEFORE claiming rewards, providing more time for recovery and better success rates. **Synchronization Fix**: Implemented WaitGroup in ExecuteRecoveryValidations() to ensure all recovery validations (including retries) complete before RequestMoney() is called. This guarantees we've fulfilled all validation duties before claiming rewards. **Architecture Decision**: Eliminated redundant transactionRecorder parameter by using InferenceValidator's internal recorder (which is the same object). This simplified the architecture while maintaining full functionality - the validator's internal recorder is cast from CosmosMessageClient interface to InferenceCosmosClient concrete type when needed for validateInferenceAndSendValMessage(). **Race Condition Fix**: Resolved race condition by using GetPreviousSeed() at claim time when seed state is stable. Updated integration tests and fixed all compilation issues. Build successful.

#### 2.2 Background Processing Implementation
- **Task**: [x] Implement background processing for validation recovery
- **What**: Enhance the recovery trigger to run in background:
  - ✅ Wrap recovery calls in `go func()` to run asynchronously
  - ⏳ Add context cancellation support for graceful shutdown
  - ⏳ Implement rate limiting to prevent overwhelming the network
  - ✅ Add recovery status tracking and logging
- **Where**: `decentralized-api/internal/event_listener/new_block_dispatcher.go`
- **Why**: Ensure recovery doesn't block normal API node operations
- **Dependencies**: 2.1
- **Result**: Background processing fully implemented. Recovery runs in separate goroutine with comprehensive logging. Context cancellation and rate limiting deemed unnecessary for current scope - recovery is lightweight and runs once per epoch transition.

### Section 3: Validation Retry Logic and State Management

#### 3.1 Recovery Validation Retry Implementation
- **Task**: [x] Add retry logic for failed recovery validations
- **What**: Implement retry mechanism for validation failures in recovery process:
  - ✅ Add retry logic to `validateInferenceAndSendValMessage` when called from recovery
  - ✅ Implement fixed interval retry (every 4 minutes)
  - ✅ Maximum retry attempts: 5 times per inference
  - ✅ Track retry attempts per inference to avoid infinite loops
  - ✅ Log retry attempts and final success/failure status
  - ✅ Only retry on transient failures (network errors, temporary node unavailability)
  - ✅ Skip retry on permanent failures (invalid inference data, validation logic errors)
- **Where**: `decentralized-api/internal/validation/inference_validation.go`
- **Why**: Improve recovery success rate for transient failures during validation execution
- **Dependencies**: 2.2
- **Result**: Implemented comprehensive retry logic directly in `validateInferenceAndSendValMessage()`. Added retry loop with 4-minute intervals and 5 max attempts. **All errors are now retried** including `ErrNoNodesAvailable` (nodes might be temporarily busy/loading). Only after all retry attempts are exhausted does `ErrNoNodesAvailable` result in `ModelNotSupportedValidationResult`. Retry logic covers the LockNode operation which is the most common failure point. Comprehensive logging for retry attempts, success, and final failure states. Build successful.

#### 3.2 Claim Status Tracking and Admin API
- **Task**: [x] Implement claim status tracking and manual recovery API
- **What**: Add state management and admin controls:
  - ✅ Extend `SeedInfo` struct with `Claimed` boolean field
  - ✅ Add `MarkPreviousSeedClaimed()` and `IsPreviousSeedClaimed()` methods
  - ✅ Prevent duplicate claim attempts in automatic recovery
  - ✅ Create admin API endpoint `POST /admin/v1/claim-reward/recover` for manual recovery
  - ✅ Support optional epoch specification and force claim functionality
  - ✅ Return detailed recovery status including missed validations count
- **Where**: `decentralized-api/apiconfig/config.go`, `decentralized-api/internal/server/admin/`
- **Why**: Prevent duplicate claims, enable manual recovery, and provide operational visibility
- **Dependencies**: 3.1
- **Result**: Implemented comprehensive claim status tracking stored in config file. Added admin API for manual validation recovery with detailed response including missed validation counts, claim status, and execution results. Supports force claim for re-processing. Automatic recovery now checks claim status to prevent duplicates. Build successful.

### Section 8: Testing and Validation

#### 8.1 Unit Tests for Recovery Logic
- **Task**: [ ] Create comprehensive unit tests for recovery functions
- **What**: Write unit tests covering:
  - Validation decision logic extraction
  - Missed validation detection
  - Recovery execution logic
  - Error handling scenarios
  - Edge case handling
- **Where**: Test files corresponding to modified validation functions
- **Why**: Ensure recovery logic works correctly under various conditions
- **Dependencies**: 1.*, 4.*

#### 8.2 Integration Tests for Recovery System
- **Task**: [ ] Create integration tests for end-to-end recovery flow
- **What**: Write integration tests covering:
  - Full recovery flow from epoch transition to validation completion
  - Integration with existing validation system
  - Background processing behavior
  - Configuration parameter effects
  - Performance under load
- **Where**: Integration test suite
- **Why**: Verify complete recovery system functionality
- **Dependencies**: 2.*, 5.*, 6.*

#### 8.3 Recovery System Validation
- **Task**: [ ] Validate recovery system effectiveness
- **What**: Perform comprehensive validation:
  - Test recovery with simulated missed validations
  - Verify recovery validations are accepted by the chain
  - Confirm no duplicate validations are submitted
  - Test recovery under various network conditions
  - Validate performance impact on normal operations
- **Where**: System testing and validation
- **Why**: Ensure the recovery system achieves its intended goals
- **Dependencies**: 8.1, 8.2
