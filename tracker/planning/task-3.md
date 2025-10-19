# Task 3: Inference Statistics Backend API

## Task
Implement REST API backend for collecting and serving inference statistics with height-wise caching and background polling.

## Result
Working backend service with:
- `/v1/inference/current` - current epoch statistics (refreshed every 30s)
- `/v1/inference/epochs/{epoch_id}?height=X` - historical epoch statistics at specific height (optional)
- Height-wise SQLite caching layer for immutable historical data
- Multi-URL client with automatic failover
- Background polling task running every 30 seconds
- All chain-api/chain-rpc requests with explicit height parameter
- Pagination limit=10000 for all participant queries

## Structure
```
backend/
├── src/backend/
│   ├── client.py         # GonkaClient - HTTP client with failover
│   ├── database.py       # CacheDB - SQLite caching layer
│   ├── models.py         # Pydantic models
│   ├── service.py        # InferenceService - business logic
│   ├── router.py         # FastAPI endpoints
│   └── app.py            # Application with background polling
├── src/tests/
│   ├── test_client.py
│   ├── test_database.py
│   ├── test_models.py
│   ├── test_service.py
│   └── test_inference_api.py
├── test_data/           # Recorded API responses
└── scripts/
    └── record_api_data.py  # Script to record live API data
```

## Approach

### Phase 1: Client & Data Recording
- Created `GonkaClient` with multi-URL failover
- Implemented methods for fetching epoch participants and participant stats
- Always use `pagination.limit=10000` for chain-api queries
- All chain-api/chain-rpc calls include explicit height parameter
- Recorded sample API responses for offline testing

### Phase 2: Database Layer
- Implemented SQLite database with two tables:
  - `inference_stats`: stores participant statistics per epoch/height
  - `epoch_status`: tracks finished epochs for immutability
- Once epoch marked finished, data never refetched
- Cache key: (epoch_id, participant_index)

### Phase 3: Models Layer
- Created Pydantic models for API contracts
- `ParticipantStats` with computed `missed_rate` field
- Formula: `missed_requests / (missed_requests + inference_count)`
- `InferenceResponse` for API responses with epoch metadata

### Phase 4: Service Layer
- `get_current_epoch_stats()`: fetch latest data, detect epoch transitions
- `get_historical_epoch_stats(epoch_id)`: check cache or fetch at height-20
- For finished epoch N: fetch data at PoC Start Height (N+1) - 20
- Automatic epoch transition detection and marking

### Phase 5: API Layer
- REST endpoints with proper error handling
- OpenAPI documentation with response models
- HTTP status codes for different error cases

### Phase 6: Background Polling
- FastAPI lifespan context manager for initialization
- Background task polling every 30 seconds
- Graceful shutdown on application stop

### Phase 7: Configuration
- Environment variables: `INFERENCE_URLS`, `CACHE_DB_PATH`
- Default URL: `http://node2.gonka.ai:8000`
- Configuration template updated

## Key Implementation Details

### Height-wise Caching (Fundamental Layer)
**Critical requirement**: All data cached per (epoch_id, height) pair
- Current epoch: always fetch at latest height
- Historical epoch N (no height specified): fetch at height = **effective_block_height(N+1) - 10**
- Historical epoch N (with height specified): fetch at requested height
- Once epoch finished and cached at canonical height, never refetch that data
- Stats CAN differ at different heights within same epoch
- **Height validation**: Rejects requests for heights before epoch start (height < effective_block_height)

### Explicit Height Requirements
**All requests must include height specification**:
- Chain-API requests: Use header `X-Cosmos-Block-Height: <height>` + query `?pagination.limit=10000`
- Chain-RPC requests: Use native height query (status endpoint)
- Current epoch: use latest height from chain
- Historical epoch: use effective_block_height(N+1) - 10 OR user-specified height
- **Critical**: Must use HTTP header (not query param) for chain-api height specification

### Epoch Transition Tracking
**Automatic detection of "old" epochs**:
- Service tracks current epoch ID
- When epoch ID changes: previous epoch becomes "old/finished"
- For finished epochs: data at canonical height (poc_start_height - 20) is immutable
- Once epoch changed, no need to refetch canonical data (it'll be the same)
- Finished epochs can still be queried at different heights if needed

### Pagination Requirements
**All chain-api participant queries**:
- Must use `?pagination.limit=10000` 
- Required because default pagination is too small
- Ensures all participants fetched in single request

### Multi-URL Failover
- Initialize with configured URLs from INFERENCE_URLS env var
- Discover additional URLs from active participants
- Automatic rotation on request failure
- Retry with all available URLs before giving up

## Testing
All components tested independently:
- Unit tests with mocked data (database, models)
- Integration tests with live Gonka Chain API
- API tests with TestClient
- 32 tests, all passing

## API Endpoints

### Current Epoch Stats
`GET /v1/inference/current?reload=false`

Returns current epoch statistics at latest height.

**Caching behavior**:
- Returns cached data if less than 30 seconds old (instant response)
- Add `?reload=true` to force fresh fetch from chain
- Background task auto-refreshes cache every 30 seconds
- This ensures fast API response while keeping data fresh

### Historical Epoch Stats
`GET /v1/inference/epochs/{epoch_id}?height=X` (height is optional)

**Without height parameter**: Returns cached canonical data (at poc_start_height(N+1) - 20)
- For finished epochs: returns immutable cached data
- For current/recent epochs: fetches at canonical height

**With height parameter**: Returns data at specific height
- Allows querying same epoch at different heights
- Stats can differ between heights within same epoch
- Useful for tracking how stats evolved during epoch
- **Validation**: Height must be >= epoch effective_block_height (rejects with 400 if before epoch start)

## API Response Format
```json
{
  "epoch_id": 56,
  "height": 887711,
  "participants": [
    {
      "index": "gonka1...",
      "address": "gonka1...",
      "weight": 37778,
      "current_epoch_stats": {
        "inference_count": "100",
        "missed_requests": "5",
        "validated_inferences": "95",
        "invalidated_inferences": "5"
      },
      "missed_rate": 0.0476
    }
  ],
  "cached_at": "2025-10-19T12:00:00Z",
  "is_current": true
}
```

**Note**: The `height` field in response indicates which height this data was fetched from.

