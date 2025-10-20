# Task 12: Timeline Feature

## Task
Add timeline feature with block explorer and network events tracking. Backend provides block timing data and events, frontend adds page navigation and interactive timeline visualization.

## Status
COMPLETED

## Result
Timeline feature implemented with:
- Backend endpoint `/v1/timeline` providing block statistics and events
- Frontend page navigation between Host Dashboard and Timeline
- Interactive block slider for time estimation
- Events list showing past and future network events
- Dynamic event data from restrictions params endpoint

## Implementation

### Backend

**Models:**
- Added `BlockInfo` model with height and timestamp
- Added `TimelineEvent` model with block_height, description, and occurred flag
- Added `TimelineResponse` model with:
  - current_block, reference_block
  - avg_block_time
  - events list
  - current_epoch_start (for calculating epoch boundaries)
  - current_epoch_index (for labeling epochs correctly)
  - epoch_length (for calculating epoch boundaries)

**Client:**
- Added `get_block(height: int)` method to fetch block data from chain-rpc
- Added `get_restrictions_params()` method to fetch restrictions parameters

**Service:**
- Added `get_timeline()` method to InferenceService
- Fetches current block and reference block (current - 10000)
- Calculates average block time between two blocks
- Fetches restrictions end block from chain-api for "Money Transfer Enabled" event
- Fetches latest epoch info for current_epoch_start, current_epoch_index, and epoch_length
- Returns timeline data with block stats, events, and epoch information

**Router:**
- Added `/v1/timeline` endpoint returning TimelineResponse

### Frontend

**Types:**
- Added `BlockInfo` interface
- Added `TimelineEvent` interface
- Added `TimelineResponse` interface

**Components:**
- Created `Timeline.tsx` component with:
  - Chain statistics display (current block, avg block time, range with days)
  - Interactive SVG timeline graph showing blocks over time
  - Dynamic range calculation (2 months in blocks based on avg block time)
  - Auto-extends range to include furthest event
  - Epoch boundary markers:
    - Calculated from current_epoch_start ± epoch_length
    - All epochs shown as vertical lines
    - Labeled with epoch numbers only for every 3rd epoch (E54, E57, E60, etc.)
    - Interactive hover showing "Epoch X Start" with time for all epochs
    - Clickable to jump to epoch start block
  - Visual event markers as vertical lines with labels
  - Events color-coded (gray for past, blue for future)
  - Hover tooltip with context-aware content:
    - Epochs: "Epoch X Start", block, time
    - Regular blocks: Block number, time
  - Click anywhere on timeline to select block
  - Current block marker (reference block removed from display)
  - Clickable event cards below the graph
  - URL parameter handling for block selection (`?block=123456`)
  - Clean, modern visual design

**App:**
- Added page state management ('dashboard' | 'timeline')
- Added navigation buttons at top of page
- Conditional rendering based on selected page
- Minimal styling to avoid overwhelming the UI
- URL parameter handling for page routing (`?page=timeline`)
- handlePageChange function updates URL and clears relevant params
- Page state initialized from URL on mount

## Data Flow

1. Backend fetches current block from chain-rpc/status
2. Backend fetches block details for current and reference blocks
3. Backend calculates average block time across 10k blocks
4. Backend fetches restrictions end block from chain-api
5. Backend builds event list with occurrence status
6. Frontend displays block statistics and slider
7. Frontend calculates estimated times using linear interpolation
8. Frontend displays events with past/future styling

## Key Implementation Notes

1. Block time calculation uses 10,000 block window for accuracy
2. Event occurrence determined by comparing to current block height
3. Timeline range: reference block to current + at least 2 months (calculated dynamically)
4. Range automatically extended to include all events
5. Time estimation uses linear interpolation with avg block time
6. No caching or auto-refresh per requirements
7. Events dynamically sourced from restrictions params API
8. API uses `restriction_end_block` (singular) not `restrictions_end_block`
9. Past events styled in gray, future events in blue
10. Navigation uses separated button layout (Dashboard left, Timeline right)
11. URL routing support for direct page and block linking
12. Events are clickable to jump to specific blocks
13. Imports moved to top of service.py file (no function-level imports)
14. SVG-based visualization for performance and scalability
15. Interactive hover state with real-time tooltip
16. Visual hierarchy: current block most prominent, events clearly marked
17. Epoch boundaries marked with subtle gray vertical lines and epoch numbers
18. Epoch information fetched from latest epoch API (index, start, length)
19. Reference block used for calculations but not displayed on timeline
20. Epoch numbers labeled as "E{number}" for clean minimal display
21. Hover state distinguishes between epoch markers and regular blocks
22. Epoch labels shown only for every 3rd epoch (divisible by 3) to reduce clutter
23. All epoch lines remain interactive even without visible labels

## Files Modified

**Backend:**
- `backend/src/backend/models.py` - Added timeline models
- `backend/src/backend/client.py` - Added block and restrictions methods
- `backend/src/backend/service.py` - Added get_timeline method
- `backend/src/backend/router.py` - Added timeline endpoint

**Frontend:**
- `frontend/src/types/inference.ts` - Added timeline interfaces
- `frontend/src/components/Timeline.tsx` - New timeline component
- `frontend/src/App.tsx` - Added page navigation and conditional rendering

## API Response Example

```json
{
  "current_block": {
    "height": 899367,
    "timestamp": "2025-10-20T12:34:56.789Z"
  },
  "reference_block": {
    "height": 889367,
    "timestamp": "2025-10-19T08:15:23.456Z"
  },
  "avg_block_time": 2.85,
  "events": [
    {
      "block_height": 1385263,
      "description": "Money Transfer Enabled",
      "occurred": false
    }
  ]
}
```

## UI Features

**Navigation:**
- Two buttons: "Host Dashboard" (left) and "Timeline" (right)
- Separated layout using flexbox justify-between
- Active page has dark background (gray-900)
- Inactive has white background with border
- Smooth transition on hover

**Timeline Page:**
- Chain statistics card with current block, avg block time, and block range
- Interactive timeline graph (SVG-based)
  - Horizontal axis representing block progression
  - Vertical event markers with labels
  - Current block indicator (black line)
  - Reference block indicator (gray line)
  - Hover state showing block and time at cursor position
  - Click to select and share specific blocks
- Events list card below graph
  - Color-coded cards (gray for past, blue for future)
  - Clickable to jump to event block

**URL Routing:**
- `/?page=timeline` - Direct link to timeline page
- `/?page=timeline&block=1385263` - Direct link to specific block
- URL updates automatically when slider moves
- URL updates when clicking events
- Dashboard clears timeline parameters

**Timeline Graph:**
- SVG-based interactive visualization
- Horizontal timeline from reference block to current + 2 months
- Auto-extends to include all events in view
- Visual markers:
  - Epoch boundaries: Light gray vertical lines with epoch numbers (E57, E58, etc.)
  - Current block: Black bold vertical line
  - Events: Colored dashed vertical lines with dots
  - Hover: Orange vertical line at cursor
- Interactive epoch markers:
  - Hoverable to see "Epoch X Start" with block and time
  - Clickable to select that epoch start block
  - Labeled with "E{number}" below the line (every 3rd epoch only)
  - All epoch lines are interactive, even those without labels
- Tooltip on hover shows:
  - For epochs: "Epoch X Start", block number, UTC time
  - For regular blocks: Block number, UTC time
- Click anywhere to select block (updates URL)
- Events display name above and block number below line
- Timeline range displays approximate days covered

**Events Display:**
- Each event in colored card (gray for past, blue for future)
- Badge showing "PAST" or "FUTURE" status
- Block height and estimated time displayed
- Event description prominent
- Clickable cards with hover effect
- Clicking event updates slider to that block

**Example URLs:**
- Direct link to timeline: `http://localhost/?page=timeline`
- Direct link to Money Transfer event: `http://localhost/?page=timeline&block=1385263`
- Direct link to current block: `http://localhost/?page=timeline&block=899483`

