# Invalid Participant Exclusion – Execution Plan (Updated: use collection directly, no helpers)

This plan implements the InvalidatedParticipants feature using the collections API directly at call sites — no keeper helper methods. Steps list concrete file touch‑points, acceptance criteria, and unit testing tasks.

Notes and decisions from the spec review:
- New chain RPC: QueryInvalidatedParticipants(epoch_id). If epoch_id == 0, default to the current epoch.
- Reason is a free‑form string.
- DAPI must not modify signed bytes; instead, set Weight = 0 in the JSON DTO for any invalidated participant. Keep ActiveParticipantsBytes unchanged.
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

## 1) Proto: InvalidatedParticipants types and query
Files
- inference-chain/proto/inference/inference/invalidated_participant.proto (new)
- inference-chain/proto/inference/inference/query.proto (edit)

Implementation
- [ ] TODO: Add invalidated_participant.proto with:
  - message InvalidatedParticipant { string address = 1; uint64 epoch_id = 2; string reason = 3; }
- [ ] TODO: In query.proto, add:
  - rpc InvalidatedParticipants(QueryInvalidatedParticipantsRequest) returns (QueryInvalidatedParticipantsResponse) { GET /productscience/inference/inference/invalidated_participants/{epoch_id} }
  - message QueryInvalidatedParticipantsRequest { uint64 epoch_id = 1; }
  - message QueryInvalidatedParticipantsResponse { repeated InvalidatedParticipant items = 1; }
- [ ] TODO: Ensure appropriate go_package and imports registered.
- [ ] TODO: Generate protobufs: from inference-chain dir, run: `ignite generate proto-go`

Unit tests (compilation-level)
- [ ] TODO: `cd inference-chain && go build ./...` succeeds

---

## 2) Keeper wiring: collection only (NO helpers)
Files
- inference-chain/x/inference/keeper/keeper.go (edit)

Implementation
- [ ] TODO: Register new collection in Keeper:
  - InvalidatedParticipants: collections.Map[collections.Pair[uint64, sdk.AccAddress], types.InvalidatedParticipant]
- [ ] NOTE: No helper methods (no Add/Is/List). Call sites must read/write the collection directly.

Unit tests
- [ ] TODO: Build/test that keeper compiles with the new collection

---

## 3) Query server: InvalidatedParticipants (direct collection access)
Files
- inference-chain/x/inference/keeper/query_invalidated_participants.go (new)

Implementation
- [ ] TODO: Implement gRPC method:
  - If request.epoch_id == 0, resolve current effective epoch.
  - Iterate collection with prefixed range by epoch_id, return list.
- [ ] TODO: Wire in module AppModule RegisterServices if needed (usually auto via proto codegen scaffolding).

Unit tests
- [ ] TODO: query_invalidated_participants_test.go – happy path, epoch_id==0 defaulting, empty list
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 4) Hook invalidation flow to populate collection (direct write)
Files
- inference-chain/x/inference/keeper/msg_server_invalidate_inference.go (edit)

Implementation
- [ ] TODO: In InvalidateInference, after calculating executor.Status, detect transition original->INVALID and write directly:
  - k.InvalidatedParticipants.Set(ctx, collections.Join(effectiveEpoch.Index, addr), types.InvalidatedParticipant{ Address: addr, EpochId: effectiveEpoch.Index, Reason: reason })
- [ ] TODO: Ensure idempotency (Set on same key overwrites)

Unit tests
- [ ] TODO: Extend existing msg_server_invalidate_inference_test.go to assert entry creation on transition to INVALID
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 5) Epoch lifecycle: clear per‑epoch invalidations at epoch switch (direct remove)
Files
- inference-chain/x/inference/module/module.go (edit moveUpcomingToEffectiveGroup)

Implementation
- [ ] TODO: At the same place ActiveInvalidations is cleared, iterate InvalidatedParticipants prefix for prior epoch and delete entries

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

## 7) Executor selection: exclude invalidated at read/selection time (defense‑in‑depth)
Files
- inference-chain/x/inference/keeper/query_get_random_executor.go (edit)

Implementation
- [ ] TODO: In both branches of createFilterFn (inference and PoC), obtain the effective epoch id and filter out members whose addresses are present in InvalidatedParticipants (scan prefix set for that epoch)

Unit tests
- [ ] TODO: Add a test ensuring GetRandomExecutor never returns an invalidated participant
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 8) Consensus/governance power: verify exclusion
Files
- inference-chain/x/inference/module/module.go (review+possible edit)

Implementation
- [ ] TODO: Before registering validators/power, deterministically drop invalidated participants (or ensure Weight=0 path leads to zero power). Keep logic deterministic and map‑free iteration.

Unit tests
- [ ] TODO: Add a unit test that an invalidated participant has zero Tendermint power and no governance voting weight
- [ ] TODO: Run: `cd inference-chain && go test ./x/inference/...`

---

## 9) DAPI: set Weight=0 for invalidated participants in DTO
Files
- decentralized-api/internal/server/public/get_participants_handler.go (edit)
- decentralized-api/internal/server/public/entities.go (verify DTO)

Implementation
- [ ] TODO: After unmarshalling ActiveParticipants for a given epoch, query chain QueryInvalidatedParticipants(epoch). Build a set of invalidated addresses.
- [ ] TODO: For each participant in activeParticipants.Participants, if address is in the set, set participant.Weight = 0 in the JSON DTO only. Do NOT modify ActiveParticipantsBytes or ProofOps.

Unit tests
- [ ] TODO: Add a unit test asserting that invalidated participants have Weight=0 in JSON, while bytes/proof remain unchanged
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
- Chain exposes QueryInvalidatedParticipants(epoch_id) with epoch_id==0 mapping to current epoch; returns address+epoch_id+reason.
- Invalidated participants are recorded upon status transition to INVALID, cleared on epoch switch, and physically removed from all model EpochGroups for the current epoch.
- Random executor and any model membership dependent logic never select invalidated participants.
- DAPI JSON sets Weight=0 for invalidated participants while preserving the signed ActiveParticipantsBytes and proofs.
- All existing and new unit tests pass; no data migrations; deterministic behavior maintained.
