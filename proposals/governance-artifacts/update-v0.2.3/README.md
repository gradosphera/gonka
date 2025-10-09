# Upgrade Proposal: v0.2.3

This document outlines the proposed changes for on-chain software upgrade v0.2.3. The `Changes` section details the major modifications, and the `Upgrade Plan` section describes the process for applying these changes.

## Upgrade Plan

This PR updates the code for the `api` and `node` services and modifies the container versions in `deploy/join/docker-compose.yml`.

The binary versions will be updated via an on-chain upgrade proposal. For more information on the upgrade process, refer to `/docs/upgrades.md`.

Existing participants are **not** required to upgrade their `api` and `node` containers. The updated container versions are intended for new participants who join after the on-chain upgrade is complete.

**Proposed Process:**
1. Active participants review this proposal on GitHub.
2. Once the PR is approved by a majority, a `v0.2.3` release will be created from this branch, and an on-chain upgrade proposal for this version will be submitted.
3. If the on-chain proposal is approved, this PR will be merged immediately after the upgrade is executed on-chain.

Creating the release from this branch (instead of `main`) minimizes the time that the `/deploy/join/` directory on the `main` branch contains container versions that do not match the on-chain binary versions, ensuring a smoother onboarding experience for new participants.

The changes in `proxy` and `proxy-ssl` services can be applied asynchronously, off-chain.


## Testing

### Testnet

The on-chain upgrade from version `v0.2.0` to `v0.2.2` and then to `v0.2.3`  has been successfully deployed and verified on the testnet.

We encourage all reviewers to request access to our testnet environment to validate the upgrade. Alternatively, reviewers can test the on-chain upgrade process on their own private testnets.


## Changes

### Replace listening to Tx events with querying blocks for event data (`5830799a390a5303445c0515d1db28ba5d943dcb`)

All changes are contained in the `decentralized-api` package. Previously, the system listened to all `Tx` events from `inference` and `bls` modules, as well as any `/cosmos.authz.v1beta1.MsgExec` transactions. This approach proved unreliable when blocks with large transaction counts are committed, causing subscription channel overflow on the sender side (Cosmos SDK node). This overflow results in subscription termination and failure to deliver future `Tx` events unless the API node is restarted.

The updated implementation maintains only subscriptions to `NewBlock` events and queries block contents separately.

Key changes:

- Event listener now has a single subscription: `tendermint/event/NewBlock`
- Each new block pushes a height update to `BlockObserver` in `block_observer.go`
- `BlockObserver` queries events for incoming blocks and tracks the last processed block height
- `Tx` event worker pool still initializes in `event_listener.go` but reads from `BlockObserver` event queue
- Special barrier events act as delivery notification mechanism for all events in a given block


### Reproducible seed from Epoch Signature (`1fe45b447b070b31fbb7ead59c60634f39116386`)

The fully random seed is replaced with a reproducible seed derived from the signature of the current epoch index. This prevents missed seeds in future epochs.

### Certik Audit fixes (`5596c5765c0b449de69f3d6ca03125fa9ff4f63e`)

Batch of minor fixes addressing Certik audit findings.

### Batch processing of inferences on validation (`49a2976e19379c5935c99063862c9652039bfa5d`)

Fixed a bug where the chain failed to process inference batches under high load conditions.

### Auto re-query of unclaimed rewards (`52796f18e2b00579f901eb9576a715169e3bfc54`)

Implemented automatic re-querying mechanism for rewards that were not claimed on initial attempt.

### cosmos-sdk cleanup (`c4592d6141232b7677c4d940ac1933aa5d1fe16c`)

Bumps the forked cosmos-sdk dependency from `v0.53.3-ps5` to `v0.53.3-ps8` (now hosted under `github.com/gonka-ai/cosmos-sdk`). This version includes code cleanup where significant portions of the staking module were removed as they are no longer needed for the Gonka chain. See https://github.com/gonka-ai/cosmos-sdk/pull/6


### proxy and proxy-ssl services (`80437507ea40eddd67c518074675f9c7970eb627`)

Introduced a new `proxy-ssl` service for automated TLS certificate management and enhanced the `proxy` service with SSL/HTTPS support.

**proxy-ssl service:**
- Issues TLS certificates via ACME DNS-01 (Let's Encrypt) for subdomains of a single base domain
- Requests require JWT authorization (`CERT_ISSUER_JWT_SECRET`)
- Only subdomains listed in `CERT_ISSUER_ALLOWED_SUBDOMAINS` under `CERT_ISSUER_DOMAIN` are permitted
- Supports Route53, Cloudflare, Google Cloud DNS, Azure DNS, DigitalOcean DNS, Hetzner DNS
- Certificate bundles written to `cert_storage_path` (default `/app/certs`)
- Runs in disabled mode if configuration is missing/invalid (serves only `/health`)

**proxy service updates:**
- Multi-mode operation: `NGINX_MODE` supports `http`, `https`, or `both`
- Automated SSL setup via `setup-ssl.sh` script with integration to `proxy-ssl` service
- Background certificate renewal loop (configurable via `RENEW_BEFORE_DAYS`)
- Alternative manual certificate support via `SSL_CERT_SOURCE` mount
- Unified `nginx.unified.conf.template` configuration
- Consistent `KEY_NAME_PREFIX` support across all upstream services
- Added `Authorization` header forwarding
- Corrected gRPC proxy directives to use `grpc_*` instead of `proxy_*`
- Configurable DNS resolver for dynamic upstream re-resolution

See `proxy/README.md` and `proxy-ssl/README.md` for detailed configuration options.
