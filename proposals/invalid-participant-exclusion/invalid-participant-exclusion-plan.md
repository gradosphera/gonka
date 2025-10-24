# Invalid Participant Exclusion – Execution Plan

This plan implements the ExcludedParticipants feature using the collections API directly at call sites — no keeper helper methods. Steps list concrete file touch‑points, acceptance criteria, and unit testing tasks.

Notes and decisions from the spec review:
- New chain RPC: QueryExcludedParticipants(epoch_id). If epoch_id == 0, default to the current epoch.
- Reason is a free‑form string; include EffectiveHeight (block height when added to the exclusion list).
- DAPI must not modify signed bytes; return a separate top‑level excludedParticipants list in the GetActiveParticipants response; do not add per‑participant flags. Keep ActiveParticipantsBytes unchanged.
- Physically remove invalidated participants from all model EpochGroup memberships (current epoch only).
- Focus on unit tests only (no integration tests in this task).

Upgrade safety and determinism:
- No data migrations; add new collection and query only.
- Deterministic behavior only; avoid map iteration in consensus paths.

---

## 0) Pre‑flight
- [ ] TODO: Create a working branch for this proposal.
- [ ] TODO: Ensure Go 1.22.x toolchain and proto toolchain available.
- [ ] TODO: Read dev guidelines in proposals/invalid-participant-exclusion/README.md and .junie/guidelines.md.

Verification
- [ ] TODO: `go env` and `go version` recorded

---

## 1) Proto: ExcludedParticipants types and query
Files
- inference-chain/proto/inference/inference/excluded_participant.proto (new)
- inference-chain/proto/inference/inference/query.proto (edit)

Implementation
- [x] TODO: Add excluded_participant.proto with:
  - message ExcludedParticipant { string address = 1; uint64 epoch_id = 2; string reason = 3; uint64 effective_height = 4; } — added file and message definition.
- [x] TODO: In query.proto, add:
  - rpc ExcludedParticipants(QueryExcludedParticipantsRequest) returns (QueryExcludedParticipantsResponse) { GET /productscience/inference/inference/excluded_participants/{epoch_id} } — added.
  - message QueryExcludedParticipantsRequest { uint64 epoch_id = 1; } — added.
  - message QueryExcludedParticipantsResponse { repeated ExcludedParticipant items = 1; } — added.
- [x] TODO: Ensure appropriate go_package and imports registered. — done, imported excluded_participant.proto.
- [x] TODO: Generate protobufs: from inference-chain dir, run: `ignite generate proto-go` — generated successfully.

Unit tests (compilation-level)
- [x] TODO: `cd inference-chain && go build ./...` succeeds — build succeeded.

---

## 2) Keeper wiring: collection only (NO helpers)
Files
- inference-chain/x/inference/keeper/keeper.go (edit)

Implementation
- [ ] TODO: Register new collection in Keeper:
  - ExcludedParticipants: collections.Map[collections.Pair[uint64, sdk.AccAddress], types.ExcludedParticipant]
- [ ] NOTE: No helper methods (no Add/Is/List). Call sites must read/write the collection directly.

Unit tests
- [ ] TODO: Build/test that keeper compiles with the new collection

---

## 3) Query server: ExcludedParticipants (direct collection access)
Files
- inference-chain/x/inference/keeper/query_excluded_participants.go (new)

Implementation
- [ ] TODO: Implement gRPC method:
  - If request.epoch_id == 0, resolve current effective epoch.
  - Iterate collection with prefixed range by epoch_id, return list.
- [ ] TODO: Wire in module AppModule RegisterServices if needed (usually auto via proto codegen scaffolding).

Unit tests
- [ ] TODO: query_excluded_participants_test.go – happy path, epoch_id==0 defaulting, empty list
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 4) Hook invalidation flow to populate collection (direct write)
Files
- inference-chain/x/inference/keeper/msg_server_invalidate_inference.go (edit)

Implementation
- [ ] TODO: In InvalidateInference, after calculating executor.Status, detect transition original->INVALID and write directly:
  - k.ExcludedParticipants.Set(ctx, collections.Join(effectiveEpoch.Index, addr), types.ExcludedParticipant{ Address: addr, EpochId: effectiveEpoch.Index, Reason: reason /* 'invalidated' */, EffectiveHeight: uint64(ctx.BlockHeight()) })
- [ ] TODO: Ensure idempotency (Set on same key overwrites)

Unit tests
- [ ] TODO: Extend existing msg_server_invalidate_inference_test.go to assert entry creation on transition to INVALID
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 5) Epoch lifecycle: clear per‑epoch exclusions at epoch switch (direct remove)
Files
- inference-chain/x/inference/module/module.go (edit moveUpcomingToEffectiveGroup)

Implementation
- [ ] TODO: At the same place ActiveInvalidations is cleared, iterate ExcludedParticipants prefix for prior epoch and delete entries

Unit tests
- [ ] TODO: module test – seed entries, simulate epoch switch, verify collection cleared
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 6) Remove invalidated participants from all model EpochGroups (state mutation)
Files
- inference-chain/x/inference/epochgroup/... (helper, new or existing)
- inference-chain/x/inference/module/module.go or keeper call site

Implementation
- [ ] TODO: Implement helper to remove a participant from the EpochGroup parent and all model subgroups for the current epoch (no schema changes; use existing group APIs/collections)
- [ ] TODO: Invoke this helper during invalidation (same hook as step 4), ensuring global removal across all models the participant serves

Unit tests
- [ ] TODO: Test that a participant present in multiple model groups is removed from all after invalidation
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 7) Executor selection: select directly from EpochGroup members (no invalidation filtering)
Files
- inference-chain/x/inference/keeper/query_get_random_executor.go (edit)

Implementation
- [ ] TODO: Ensure GetRandomExecutor selects from the EpochGroup membership as-is; do not perform any invalidation-based filtering in createFilterFn or elsewhere.
- [ ] NOTE: Invalidated participants must be fully removed from the EpochGroup parent and all model sub-groups at invalidation time (see step 6), so no filtering is required.

Unit tests
- [ ] TODO: Add a test ensuring GetRandomExecutor never returns an invalidated participant (because the member has been removed from the EpochGroup)
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 8) Consensus/governance power: verify exclusion
Files
- inference-chain/x/inference/module/module.go (review+possible edit)

Implementation
- [ ] TODO: Before registering validators/power, deterministically drop only participants whose invalidation status is true for the current epoch. Participants excluded for other reasons must retain governance voting and Tendermint consensus power. Do not use the ExcludedParticipants list for this decision. Keep logic deterministic and map‑free iteration.

Unit tests
- [ ] TODO: Add a unit test that an invalidated participant has zero Tendermint power and no governance voting weight
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 9) DAPI: add top-level excludedParticipants in GetActiveParticipants response
Files
- decentralized-api/internal/server/public/get_participants_handler.go (edit)
- decentralized-api/internal/server/public/entities.go (verify DTO)

Implementation
- [ ] TODO: After unmarshalling ActiveParticipants for a given epoch, query chain QueryExcludedParticipants(epoch) and build the excludedParticipants list from all returned entries (no filtering by reason).
- [ ] TODO: Add this list as a separate top-level field excludedParticipants in the JSON response. Do NOT add per-participant flags and do NOT modify ActiveParticipantsBytes or ProofOps.

Unit tests
- [ ] TODO: Add a unit test asserting that the response includes a top-level excludedParticipants list (for the current epoch) and that ActiveParticipantsBytes and ProofOps remain unchanged
- [ ] TODO: Run: `cd decentralized-api && go test ./...`

---

## 10) Documentation and housekeeping
Files
- proposals/invalid-participant-exclusion/README.md (no change; reference plan)
- dev_notes/* (optional)

Implementation
- [ ] TODO: Add comments near new RPC and collection explaining epoch scoping and upgrade safety
- [ ] TODO: Note to operators: no migration; new collection only

Verification
- [ ] TODO: `make local-build` still succeeds (no integration tests)
- [ ] TODO: `cd inference-chain && go test ./...` passes
- [ ] TODO: `cd decentralized-api && go test ./...` passes

---

## Acceptance Criteria
- Chain exposes QueryExcludedParticipants(epoch_id) with epoch_id==0 mapping to current epoch; returns address+epoch_id+reason+effective_height.
- On invalidation status transition to INVALID, an ExcludedParticipants entry is recorded for the current epoch; entries are cleared on epoch switch; and the participant is physically removed from all model EpochGroups for the current epoch.
- GetRandomExecutor selects from EpochGroup membership without any invalidation-based filtering; invalidated participants cannot be selected because they are fully removed from the EpochGroup (parent and sub-groups) at invalidation time.
- Consensus/governance power removal is driven by invalidation status only; ExcludedParticipants list is not consulted for this decision.
- DAPI JSON includes a top‑level excludedParticipants list for the current epoch while preserving the signed ActiveParticipantsBytes and proofs (no per‑participant flags or weight changes).
- All existing and new unit tests pass; no data migrations; deterministic behavior maintained.
