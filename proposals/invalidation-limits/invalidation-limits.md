# Validation Invalidation Throttling

## Context

In Gonka, inference validation ensures the quality and integrity of AI outputs on a decentralized network. Participants validate inferences produced by others, with invalidations triggering broader rechecks and potential penalties.

An incident involving malformed non-English inputs triggered a validation cascade, resulting in network-wide load amplification. This exposed a serious vulnerability: bad or misconfigured participants could unintentionally or maliciously overwhelm the system with invalidation traffic.

---

## Problem Statement

### Known Vulnerabilities

1. **Unauthenticated Invalidation Requests**
   Anyone can submit invalidation messages, regardless of their role or participation in the network.

2. **Duplicate Submissions**
   Participants can validate or invalidate the same inference multiple times, skewing influence and work credit.

3. **Unlimited Parallel Invalidations**
   No cap exists on the number of simultaneous invalidations any participant can initiate.

4. **Costly Invalidation Process**
   Every invalidation triggers expensive re-validation, consensus voting, and global computation.

---

## Solution Summary

We introduce a throttling algorithm and accompanying protocol changes to:

### Restrict Who Can Invalidate

* Only **active Gonka participants with a matching model** are allowed to submit invalidation messages.

### Enforce One Validation Per Participant Per Inference

* A participant can submit **only one validation or invalidation** per inference.

### Limit Concurrent Invalidations per Participant

* Each participant is limited in how many invalidations they can trigger **in parallel**.
* The cap depends on their:

    * **Weight (W)** — based on compute contributed
    * **Reputation (R)** — based on historical reliability
    * **Recent Inferences (I)** - the inference volume

> The actual algorithm for computing this per-participant cap is defined in [`maximum_invalidations.md`](../inference-chain/x/inference/calculations/maximum_invalidations.md).

### Use a Nonlinear Scaling Function

* To prevent runaway invalidations, the number allowed per participant **curves off** as volume grows.
* We use a tunable function to gradually approach a **governance-defined maximum**.

### Behavior when invalidation cap is reached
* When a specific validator hits the cap, the inference will be neither validated nor invalidated
* The validator will still get credit for validating the inference for Claim purposes
---

## Balancing Goals

* Fast response to bad actors
* Protection under load — using adaptive throttling to prevent denial-of-service from repeated invalidations.
* Stable governance — parameters can be safely adjusted via governance without rewriting core logic.

---

## Tunable Parameters

All are chain-governed and upgradable:

| Parameter                   | Description                                           |
|-----------------------------|-------------------------------------------------------|
| `MaxInvalidations (M)`      | Global cap on simultaneous invalidations              |
| `CurveFactor (C)`           | Controls steepness of curve                           |
| `InvalidationsSamplePeriod` | Defines recent inference window to consider (seconds) |

---
### Implementation Considerations
* Need a way to track current invalidations by participant
* Failsafe of resetting current invalidations each epoch in case something during an invalidation fails (not enough votes, other issues)
* Double check that capped invalidations still work for claim money
* Leverage the same count in pricing to determine inferences throughput
* MsgRevalidateInference and MsgInvalidateInference must add the invalidator so we can track when an invalidation is finished
## Future Considerations

* False invalidation penalties
* Quarantine mode for consistently unreliable participants
* Eventual redesign to avoid full-network invalidation altogether, allowing a fraction of the network to verify

---

## Outcome

This system protects Gonka’s validation system from overload, while allowing high-quality participants to flag bad results efficiently. Through a weighted and curved limit system, invalidations remain meaningful, limited, and manageable.
