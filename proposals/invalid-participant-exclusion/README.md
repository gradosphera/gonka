# Invalid Participant Exclusion – Feature Specification

## **Overview**

This feature refines, fixes and fully tests the mechanism for handling **invalid participants** in the Gonka network. Invalid participants are nodes that have misbehaved (e.g., submitted bad inferences, misconfigured models, attempted cheating, or failed other behavioral criteria). The goal is to ensure they are **excluded from all network responsibilities and consensus mechanisms**, without retroactively altering cryptographically signed data.

---

## **Problem Statement**

Currently, the list of **active participants** retrieved from the chain **could include nodes that are technically invalid** for the current epoch. This list is **signed and committed cryptographically** each epoch, making it immutable and essential for trust and traceability via Merkle proofs.

However, since some participants may be no longer trustworthy (due to detected invalid behavior during the epoch), relying solely on the active list is not sufficient for selecting endpoints to use.

Additionally, when a participant is marked as invalid, we need to ensure and test that they are excluded from:
* Task assignment (inference or validation)
* Voting weight calculation
* Consensus power allocation
* Inference routing via the decentralized API (DAPI)
* Model group membership logic (EpochGroup)
* Clients selecting transfer agents

---

## **Proposed Solution**

### 1. **Introduce a New Query and data structure: `InvalidatedParticipants`**

* A new chain query will return a list of **invalidated participants for the current epoch** only.
* This query will include:
    * Participant identifier
    * Epoch index for when they are invalidated
    * Reason for invalidation (e.g., bad inference, wrong model, configuration issue)
* No cryptographic proof is necessary (for now) as it’s only relevant to the current epoch and used for filtering.
* The list will be added to whenever a participant is marked invalid by the validation algorithms
* There should be no need for specific pruning
* There should be no write access to the list via queries or other endpoints.

### 2. **Update DAPI Logic to Respect Invalid Participants**

* When querying for active participants via the DAPI:

    * Also query `InvalidatedParticipants`
    * Add an "invalidated" field for the value.
    * We will rely on updated clients to exclude these now invalidated participants
    * (We cannot filter at this level as clients still need the cryptographically secured list)

### 3. **Recursive Removal from All Model Group Memberships**

* An invalidated participant must be **removed from all models they serve**, not just the model they were invalidated for.
* Treat invalidation as a **global disqualification** from participation for the epoch.

### 4. **Ensure Invalid Participants Have No Voting or Consensus Power**

* Remove consensus-related influence (this is already done, but not properly verified in tests)
    * No voting rights in governance
    * No consensus power in Tendermint

---

## **Testing & Validation Plan**

* The invalidation mechanism was previously disabled during development and was under tested. Now that the full behavior is enabled:

    * Ensure that invalid participants are:
        * Properly listed in the `InvalidatedParticipants` query
        * Excluded from all responsibilities
        * Not receiving rewards, work, or assignments
        * Removed from voting and consensus mechanisms
* Extend **Testermint** tests to cover these scenarios

---

## **Client & Consumer Requirements**

* All example clients (and production consumers) must:
    * Update to use the **filter the list** from DAPI to exclude invalid participants when selecting an endpoint

---

## **Terminology Clarification**

* **Invalidated Participant**: A participant that has been deemed untrustworthy for the current epoch due to failed validations, model misalignment, or malicious behavior.
* **Active Participant**: A participant still cryptographically listed as active, but may need filtering at runtime if they’re invalidated.

