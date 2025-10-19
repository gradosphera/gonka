# Optimization: Participant Cache Pre-population

## Problem
Switching between epochs was very slow due to frontend making ~100 individual requests per epoch:
- 1 request for epoch stats
- N requests for each participant (rewards, warm keys, hardware nodes)
- Each epoch switch triggered all requests again
- Network overhead and latency made switching between epochs sluggish

## Solution
Moved cache pre-population logic from frontend to backend with async fire-and-forget pattern.

### Backend Changes

**New Method (`service.py`):**
```python
async def _ensure_participant_caches(epoch_id, participants):
    # For each participant, check and populate missing caches:
    # - Rewards (epoch performance summary)
    # - Warm keys (authz grants)
    # - Hardware nodes (MLNode data)
    # Only fetches if not already cached
```

**Triggered From:**
- `get_current_epoch_stats()` - after returning epoch data
- `get_historical_epoch_stats()` - after returning epoch data (both cached and fresh)
- Uses `asyncio.create_task()` for fire-and-forget (non-blocking)

### Frontend Changes

**Removed Pre-fetch Loop (`App.tsx`):**
- Deleted lines 41-48 that made individual participant requests
- Now only fetches epoch data
- Participant modal requests hit pre-populated caches

## Performance Impact

**Before:**
- Frontend → Backend: ~100 requests per epoch switch
- Total time: 5-10 seconds depending on network
- Visible slowdown when switching between epochs

**After:**
- Frontend → Backend: 1 request per epoch switch
- Backend populates caches in background
- Epoch switch is instant
- Participant modals open instantly (cached data)

## Technical Details

**Async Pattern:**
- Main request returns immediately with epoch data
- Background task populates all participant caches
- No blocking, no waiting
- Subsequent participant modal opens use cached data

**Cache Logic:**
- Only fetches if cache is missing
- Skips already-cached participants
- Graceful error handling per participant
- Logs debug messages for tracking

**Scope:**
- Rewards: `get_reward(epoch_id, participant_id)`
- Warm keys: `get_warm_keys(epoch_id, participant_id)`
- Hardware nodes: `get_hardware_nodes(epoch_id, participant_id)`

## Files Modified

**Backend:**
- `backend/src/backend/service.py` - added `_ensure_participant_caches()` method
- `backend/src/backend/service.py` - triggered from epoch stats methods

**Frontend:**
- `frontend/src/App.tsx` - removed pre-fetch loop

## Testing

- All existing tests pass (7 service tests)
- No breaking changes
- Cache behavior unchanged (just timing)

## Notes

- Background population happens after response sent
- User sees instant epoch switch
- Participant data ready by the time modal opens
- Reduces frontend complexity
- Centralizes cache management in backend

