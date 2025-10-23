# Development Guidelines for Gonka Project

This document outlines the development guidelines for the Gonka project, with a special focus on considerations for AI Agents (like Junie) working with the codebase.

---

## Project Overview

Gonka is a decentralized AI infrastructure designed to optimize computational power for AI model training and inference.
The project uses a novel consensus mechanism called **Proof of Compute** that ensures computational resources are allocated to AI workloads rather than wasted on securing the blockchain.

The system consists of three main components:

1. **Chain Node** — Connects to the blockchain, maintains the blockchain layer, and handles consensus.
2. **API Node** — Serves as the coordination layer between the blockchain and the AI execution environment.
3. **ML Node** — Handles AI workload execution: training, inference, and Proof of Compute validation (this is currently not in this repo).

---

## ✅ Updated: Live Chain and Upgrade Policy

The Gonka chain is **LIVE**.
This means **all changes must be upgrade-safe** and **data migration should be avoided** whenever possible.

* **Never break existing chain data or consensus logic.**
* **If state changes are required**, implement upgrade handlers in separate functions (one per change) inside the appropriate module (e.g., `inference-chain/x/<module>/module.go`).
* **Do not call these handlers directly.**
  Instead, the **user** will manually add calls to the correct upgrade migration functions in the chain’s `app/app.go` upgrade handler.
* **Avoid data migrations** unless absolutely necessary — prefer adding new collections or fields instead of mutating or restructuring existing ones.

---

## Repository Structure

```
/client-libs        # Client scripts to interact with the chain
/cosmovisor         # Cosmovisor binaries
/decentralized-api  # API node
/dev_notes          # Chain developer knowledge base
/docs               # Documentation on specific aspects of the chain
/inference-chain    # Chain node
/prepare-local      # Scripts and configs for running local chain
/testermint         # Integration test suite
/local-test-net     # Scripts and files for running a local multi-node test net
```

---

## ✅ Updated: Guidelines for Generated Files and Collections

### Protobuf Files

**IMPORTANT:** Never edit `.pb.go` files directly. These are auto-generated from `.proto` files.

When working with protobuf definitions:

1. Edit the `.proto` files.
2. Run:

   ```bash
   ignite generate proto-go
   ```

   from the `inference-chain` directory.
3. For ML node protobuf definitions, refer to the [chain-protos repository](https://github.com/product-science/chain-protos/blob/main/proto/network_node/v1/network_node.proto).
4. After editing `.proto` files, ensure they are synchronized across related repos if needed.

---

### ✅ Updated: Collections and `ignite` Usage

We **no longer use `ignite`** to create new data objects on the chain, since it does **not fully support the new Cosmos `collections` library**.

#### To add new state data (collections-based):

1. **Define your data structure in a `.proto` file.**
2. Run:

   ```bash
   ignite generate proto-go
   ```

   to generate boilerplate code.
3. **Manually register new collections** in
   `inference-chain/x/inference/keeper/keeper.go`,
   following the patterns already established for other collections-based state.
4. **Do not scaffold new stores or messages using `ignite scaffold map` or similar.**

> ⚠️ Only the `.proto` file should be modified or added.
> All collection wiring must be done manually in the keeper.

#### We still use `ignite` for:

* **Adding new queries**, since this workflow remains stable:

  ```bash
  ignite scaffold query getGameResult gameIndex --module checkers --response result
  ```

---

## Blockchain-Specific Considerations

### Avoiding Consensus Failures

Consensus failures occur when nodes calculate the state differently.
To prevent this:

1. **Do not use maps** in any deterministic state calculation.

    * Go’s map iteration order is indeterminate.
    * Use slices or deterministic ordering instead.

2. **Avoid randomness** in any chain state computation.

    * Random values, UUIDs, or timestamps must never affect consensus.

3. **Never iterate over maps** to generate state lists.

    * If needed, sort keys or use deterministic iteration.

---

### Debugging Consensus Failures

If a consensus failure occurs:

1. Note the failing block height.
2. Enter a container running a node.
3. Run:

   ```bash
   inferenced export --height <block height>
   ```
4. Compare the JSON state output from multiple nodes to locate divergence.

---

## Testing Requirements

Before submitting a pull request:

1. Run unit tests and integration tests:

   ```bash
   make local-build
   make run-tests
   ```

   > Note: `make run-tests` can take over 90 minutes.
2. Ensure all unit and integration tests pass (except known issues in `testermint/KNOWN_ISSUES.md`).

---

## Documentation

Always update documentation alongside any code change that affects:

* Behavior
* APIs
* Assumptions or chain logic

Incomplete documentation can delay PR approval.

---

## Guidelines for AI Agents (like Junie)

AI Agents contributing to this codebase must:

1. **Understand the architecture** — Know how Chain Node, API Node, and ML Node interact.
2. **Respect generated files** — Never modify `.pb.go` files.
3. **Be upgrade-aware** —

    * All changes must be compatible with a live chain.
    * Never assume full data resets are allowed.
    * Use upgrade handlers when unavoidable.
4. **Avoid non-determinism** — No maps or randomness in consensus paths.
5. **Run unit tests** — Do not skip; avoid running integration tests unless instructed.
6. **Document changes** — Explain reasoning and expected behavior clearly.
7. **Follow Cosmos SDK conventions** — Especially for module structure and handler registration.
8. **Do not commit** — Provide diffs, PR drafts, or patch instructions for human review.
9. **Be cautious with collections** — Use existing patterns in `keeper.go` for adding new collections.
10. **Be explicit about upgrades** — Include notes on whether a change requires an upgrade handler and what it does.

---

## Running Unit Tests During Development

To run tests in the `inference-chain` project:

```bash
cd inference-chain
go test ./...               # Run all tests
go test ./x/inference/...   # Run tests for a specific module
```