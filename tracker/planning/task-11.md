# Task 11: Per-Node Weight Data from Epoch Participants API

## Task
Add per-node weight data from epoch participants API to ML node information by matching node_id to local_id and storing poc_weight field.

## Status
COMPLETED

## Result
Per-node weight data (poc_weight) now captured from epoch participants API and stored per epoch per participant per node:
- Extracted from ml_nodes field in epoch participants response
- Matched by node_id to hardware node local_id
- Stored in database per epoch
- Available in participant details API response
- Works for both current and historical epochs

## Implementation

**Models:**
- Updated `MLNodeInfo` to include `poc_weight: Optional[int] = None`

**Database:**
- Added `poc_weight INTEGER` column to `participant_hardware_nodes` table
- Updated `save_hardware_nodes_batch()` to store poc_weight
- Updated `get_hardware_nodes()` to retrieve poc_weight

**Service:**
- Added `_extract_ml_nodes_map()` helper function to extract ml_nodes data from epoch participants
- Returns `Dict[str, int]` mapping node_id to poc_weight
- Updated `get_current_epoch_stats()` to extract and cache ml_nodes_map
- Updated `get_historical_epoch_stats()` to extract and cache ml_nodes_map
- Updated `get_participant_details()` to merge ml_nodes weight data with hardware nodes
- Stores ml_nodes_map in cache with `_ml_nodes_map` key

**Frontend:**
- Updated `MLNodeInfo` interface to include `poc_weight?: number`
- Updated `ParticipantModal.tsx` to display poc_weight as separate field block before Models section
- Added red highlighting for MLNodes with status "FAILED"

## Data Flow

1. Epoch participants API returns ml_nodes structure:
```json
{
  "participants": [{
    "index": "gonka14cu38x...",
    "ml_nodes": [{
      "ml_nodes": [{
        "node_id": "node8",
        "poc_weight": 1793
      }]
    }]
  }]
}
```

2. Service extracts ml_nodes_map:
```python
ml_nodes_map = _extract_ml_nodes_map(participant["ml_nodes"])
# Result: {"node8": 1793, "node9": 2455, ...}
```

3. Cached in stats with `_ml_nodes_map` key per participant

4. When fetching hardware nodes, merged by matching:
   - `node_id` (from epoch participants) == `local_id` (from hardware nodes)

5. Returned in participant details API with poc_weight field

## Testing

Created `test_mlnode_weights.py` with 14 tests:
- 8 tests for `_extract_ml_nodes_map()` helper function
  - Simple case, multiple nodes, empty, missing fields, null values, zero values
- 3 tests for `MLNodeInfo` model with poc_weight field
- 3 tests for database save/get operations with poc_weight

All 98 backend tests passing (14 new + 84 existing)

## Key Implementation Notes

1. The ml_nodes data has nested structure: `ml_nodes[0].ml_nodes[]`
2. Match by: `node_id` (from epoch participants) == `local_id` (from hardware)
3. Only poc_weight stored (timeslot_allocation not needed per user request)
4. Fields are optional - handle missing/null gracefully
5. Works for both current and historical epochs
6. Efficient: extracted once during stats fetch, cached with participant data
7. No additional API calls required
8. Merged from cache when building participant details response

## Files Modified

**Backend:**
- `backend/src/backend/models.py` - Added poc_weight to MLNodeInfo
- `backend/src/backend/database.py` - Added poc_weight column and updated methods
- `backend/src/backend/service.py` - Added extraction helper and merge logic
- `backend/src/tests/test_mlnode_weights.py` - New test file with 14 tests

**Frontend:**
- `frontend/src/types/inference.ts` - Added poc_weight to MLNodeInfo interface
- `frontend/src/components/ParticipantModal.tsx` - Added poc_weight display and FAILED status highlighting

## Migration Notes

Database schema change:
- New column `poc_weight INTEGER` added to `participant_hardware_nodes` table
- Nullable column - existing rows will have NULL
- No data migration needed
- New data will populate poc_weight automatically

Backward compatibility:
- poc_weight is optional in both backend and frontend
- Handles missing/null values gracefully
- No breaking changes to existing functionality

## UI Display

**MLNode Card Layout:**
```
node8                           [INFERENCE]

WEIGHT
1,793

MODELS
[model badges]

HARDWARE
1x NVIDIA GeForce RTX 3090

NETWORK
172.18.114.106:8080
```

**Status Colors:**
- FAILED status: Red background, red text, red border (bg-red-100 text-red-700 border-red-300)
- Other statuses: Blue background, blue text, blue border (bg-blue-100 text-blue-700 border-blue-300)

**Weight Display:**
- Shown as separate field block using same style as Hardware and Network
- Only displays when poc_weight is defined and not null
- Formatted with thousand separators for readability
- Label: "WEIGHT" (uppercase, gray)
- Value: Plain text (gray-700)

