# Proposal: Implement Governance-Controlled TrainingAllowList for the Gonka Network

## Overview
The Gonka Network is currently operational and supports inference functionality. However, AI training is not yet ready for production use and remains under active development. While training functionality is being built, it introduces risks if prematurely exposed to the network.

## Problem Statement
At present, the training code contains multiple security vulnerabilities and lacks protections on who can send training-related messages. This leaves the network open to:
- **Denial of Service attacks** via unregulated training messages.
- **Security exploits** in unfinished training code.
- **User confusion**, as some participants may mistakenly believe that training is already production-ready.

Without restrictions, malicious or unintentional use of training messages could undermine network stability and credibility.

## Proposed Solution
Introduce a governance-controlled **TrainingAllowList** for training messages:

- **Allow List Functionality**
    - Only addresses on the TrainingAllowList may send training-related messages.
    - The TrainingAllowList will be stored on-chain.
    - Any training message received will immediately check if the sender is authorized. If not, it will be rejected.

- **Governance Control**
    - The TrainingAllowList can only be updated via governance proposals, ensuring changes are transparent and community-approved.
    - Three governance message types will be introduced:
        1. **AddUserToTrainingAllowList** – Adds a new address.
        2. **RemoveUserFromTrainingAllowList** – Removes an address.
        3. **SetTrainingAllowList** – Replaces the entire allow list with a new set of addresses.

- **Security & Validation**
    - Basic validation will still apply to all training messages (e.g., format, value ranges).
    - Governance review of proposals will mitigate risks of spam, oversized messages, or malicious input.
    - Scale concerns are minimal since updates to the TrainingAllowList can only occur through governance votes.

## Benefits
- Prevents unauthorized or premature use of training functionality.
- Provides clarity to the community on what features are available.
- Mitigates denial of service and other abuse risks until training is production-ready.
- Maintains network trust and stability while development continues.  