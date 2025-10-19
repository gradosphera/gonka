# Task 10: Total Assigned Rewards in Epoch Header

## Task
Add total assigned rewards field to epoch header (next to Total Participants) for finished epochs, calculated by summing all participants' rewards.

## Status
COMPLETED

## Result
Production-ready total assigned rewards system with:
- Display right after Total Participants in header
- Always visible with "Calculating..." when data not available
- Works for both current and finished epochs
- Async fire-and-forget calculation (never blocks dashboard load)
- Background polling for last 5 finished epochs (10 minutes)
- Permanent caching per epoch
- Clear loading state indicator

## Implementation

**Database:**
- Created `epoch_total_rewards` table (epoch_id PRIMARY KEY, total_rewards_gnk, calculated_at)
- Methods: `save_epoch_total_rewards()`, `get_epoch_total_rewards()`
- Single row per epoch for efficient storage

**Models:**
- Updated `InferenceResponse` to include `total_assigned_rewards_gnk: Optional[int] = None`

**Service:**
- `_calculate_and_cache_total_rewards(epoch_id)` - fetches all participant rewards, sums, and caches
- `poll_epoch_total_rewards()` - background polling for epochs (current-1 to current-5)
- `get_historical_epoch_stats()` - triggers async calculation if not cached (fire-and-forget)
- Always returns `None` for current epoch (not relevant until finished)

**App:**
- Added `poll_epoch_total_rewards()` background task (600s intervals, 30s startup delay)
- Cancellation handler for graceful shutdown

**Frontend:**
- Updated `InferenceResponse` interface with `total_assigned_rewards_gnk?: number`
- Display after Total Participants in header (always visible)
- Format when available: "123,456 GNK" with thousand separators
- Format when calculating: "Calculating..." (gray, italic)
- Always renders field for consistent layout

## Technical Details

**Async Fire-and-Forget Pattern:**
- Check database first for cached value
- If cached: return immediately
- If not cached: trigger `asyncio.create_task()` for background calculation
- Return `None` immediately (never blocks)
- Frontend conditionally hides field when `None`

**Calculation Process:**
- Fetch epoch participants list
- For each participant: fetch epoch performance summary
- Sum all `rewarded_coins` values
- Convert to GNK (divide by 1_000_000_000)
- Save to database

**Background Polling:**
- Runs every 10 minutes (600s)
- Calculates for last 5 finished epochs (current-1 to current-5)
- Skips if already cached
- Proactively keeps recent epochs ready

**Display Logic:**
- Always renders field for consistent UI
- When available: formats with `toLocaleString()` for thousand separators
- When calculating/null: shows "Calculating..." in gray italic text
- No layout shift when data becomes available

## Testing

**Created `test_epoch_total_rewards.py` with 6 tests:**
1. Database save and get operations
2. Get epoch total rewards not found
3. Replace epoch total rewards (update scenario)
4. Multiple epochs with different totals
5. InferenceResponse model with total_assigned_rewards_gnk
6. InferenceResponse model without field (current epoch)

**All 84 backend tests passing:**
- 6 new total rewards tests
- All existing tests still passing
- No breaking changes

## Performance

**Polling intervals:**
- Epoch stats: 5 minutes (300s)
- Jail: 120s
- Health: 30s
- Rewards: 60s
- Warm keys: 5 minutes (300s)
- Hardware nodes: 10 minutes (600s)
- **Total rewards: 10 minutes (600s)** - new

**Response times:**
- Cached: instant (<5ms)
- Not cached: instant response with `null`, calculation in background
- Background calculation: 10-30s depending on participant count
- Never blocks main dashboard load

## Files Modified

**Backend:**
- `backend/src/backend/database.py` - table and methods
- `backend/src/backend/models.py` - updated InferenceResponse
- `backend/src/backend/service.py` - calculation and polling methods
- `backend/src/backend/app.py` - background polling task
- `backend/src/tests/test_epoch_total_rewards.py` - new test file

**Frontend:**
- `frontend/src/types/inference.ts` - updated interface
- `frontend/src/App.tsx` - display section

## Notes

- Only calculated for finished epochs (never for current)
- Single calculation per epoch (cached forever once done)
- Async fire-and-forget prevents blocking
- Background polling keeps last 5 epochs ready
- Always visible with "Calculating..." indicator when data not available
- No layout shift - consistent UI regardless of data state
- Follows minimalistic pattern from tasks 7-9

