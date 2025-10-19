# Task 7: Participant Reward Tracking

## Task
Add reward tracking per epoch for participants with assigned rewards, claim status, and seed signature in a new participant details endpoint.

## Status
COMPLETED

## Result
Production-ready reward tracking system with:
- New endpoint: `GET /v1/participants/{id}?epoch_id=X&height=Y`
- Rewards table showing last 5-6 epochs (GNK amounts, claim status)
- Seed signature display per participant per epoch
- Background polling for unclaimed rewards (60s, skips current epoch)
- Inline fetching with persistent database caching
- Frontend pre-fetching for instant modal opening
- Direct participant linking via URL parameters
- Current epoch detection to avoid 404 errors
- Clean separation between current/historical epoch handling


## Testing

**Backend tests:**
- Database: reward storage, retrieval, batch operations
- Client: performance_summary endpoint with height
- Service: participant details, reward polling
- Router: new endpoint with validation

**All 61 backend tests passing including:**
- 5 new tests for seed signature storage
- 3 new tests for reward database operations
- 1 new test for performance summary endpoint structure
- 4 new tests for reward/seed models

**Integration:**
- Reward polling updates claim status
- Seed extraction during epoch polling
- Participant details endpoint returns complete data
- Frontend displays rewards correctly

## Implementation

**Database:**
- Extended inference_stats with seed_signature column (reused existing table)
- Created participant_rewards table (epoch_id, participant_id, rewarded_coins, claimed)
- Methods: save_reward_batch, get_reward, get_rewards_for_participant

**Client:**
- get_epoch_performance_summary(epoch_id, participant_id, height) - fetch rewards from chain
- get_latest_epoch() - fetch current epoch info to avoid 404s

**Models:**
- RewardInfo, SeedInfo, ParticipantDetailsResponse, LatestEpochInfo

**Service:**
- Seed extraction during normal epoch polling (automatic, no separate polling)
- get_participant_details() - detects current vs historical epoch, fetches rewards inline if missing
- poll_participant_rewards() - background task, skips current epoch, checks last 6 finished epochs
- Inline fetching: missing rewards fetched and cached on first request

**Router:**
- GET /v1/participants/{id}?epoch_id=X&height=Y
- Validation: 400 for invalid params, 404 for not found, 500 for errors

**App:**
- poll_rewards task (60s, 15s startup delay)
- Epoch polling changed to 5 minutes (300s)

**Frontend:**
- ParticipantModal: seed + rewards table in Rewards section
- URL parameters: ?epoch=X&participant=gonka1abc... for direct linking
- Pre-fetching: all participants on page load (populates backend cache)
- Fixed re-fetching: split useEffect, uses participant.index instead of object

## Performance

**Polling intervals:**
- Epoch stats: 5 minutes
- Jail: 120s
- Health: 30s  
- Rewards: 60s

**Response times:**
- Participant details: ~100-500ms first time, <50ms cached
- Rewards: instant (cached or inline-fetched)
- Seeds: instant (cached with epoch data)

## Key Features

**Reward Tracking:**
- Current epoch: shows last 5 finished epochs
- Historical epoch: shows 6 epochs including viewed epoch
- Conversion: ngonka → GNK (integer division by 1B)
- Shows "-" for zero/missing rewards

**Data Guarantees:**
- Inline fetching: rewards always present on first view
- Database caching: instant response on subsequent views
- Pre-fetching: all participants cached when page loads
- Background polling: updates every 60s for unclaimed rewards

**Current Epoch Handling:**
- Detects current epoch via `/v1/epochs/latest`
- Uses current stats (fast, no epoch+1 fetch)
- Skips reward fetching (not available until epoch ends)
- Shows informative message instead of error

**URL Parameters:**
- `?epoch=56` - view specific epoch
- `?participant=gonka1abc...` - auto-open participant modal
- `?epoch=56&participant=gonka1abc...` - both
- URL updates when clicking participants (shareable links)

## Issues Fixed

**404 errors on current epoch:**
- Added get_latest_epoch() for current epoch detection
- Updated get_canonical_height() to handle current epoch without fetching epoch+1
- Updated get_participant_details() to use current stats for current epoch

**Rewards not showing:**
- Added inline reward fetching with caching
- Pre-fetch all participants when page loads
- Fixed frontend re-fetching loop (split useEffect)

**Constant UI flickering:**
- useEffect now depends on participant.index (not full object)
- Separate effect for keyboard handler

