# Increase PoC Validation Duration

This proposal changes the following parameter:
- `inference.epoch_params.poc_validation_duration` from `20` to `120`.

## Motivation

The network is growing faster than it can adapt, causing some nodes to not have enough time to validate other participants. This can make it difficult for new participants to join.
This proposal increases the validation time, which should be sufficient to support 2,000-3,000 H100 GPUs.

Longer term we can enable scale by sampling validators or decreasing the automatic adjustment valid nonce / raw nonce.