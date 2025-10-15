# Task 1: Model Management System

## Overview

Implement a ModelManager class and REST API for managing HuggingFace models with background downloads, status tracking, and cache verification.

## Architecture Decisions

### Model Identifier
- Use **JSON body** for all endpoints (simpler, avoids URL encoding issues with `/` in repo names)

### Download Concurrency
- **Limit to 3 concurrent downloads**
- Return 429 (Too Many Requests) if limit exceeded

### State Persistence
- **Downloads lost on restart** (simpler, acceptable for MVP)
- In-memory tracking only

### Background Tasks
- Use **asyncio tasks** (consistent with existing `proxy.py` and `app.py` architecture)
- Won't impact vLLM proxy performance (all async I/O-bound operations)

### Additional Features
- Status types: `DOWNLOADED`, `DOWNLOADING`, `NOT_FOUND`, `PARTIAL` (for incomplete/failed downloads)
  - `DOWNLOADED`: Model fully downloaded and verified
  - `DOWNLOADING`: Download currently in progress
  - `NOT_FOUND`: No trace of model in cache
  - `PARTIAL`: Some files exist in cache but model is incomplete (failed or cancelled)
- Include `LIST` endpoint to enumerate cached models
- Progress tracking with start time and elapsed seconds

## Implementation Details

### 1. Core Model Classes (`packages/api/src/api/models/types.py`)

```python
from pydantic import BaseModel
from typing import Optional
from enum import Enum

class Model(BaseModel):
    hf_repo: str
    hf_commit: Optional[str] = None

class ModelStatus(str, Enum):
    DOWNLOADED = "DOWNLOADED"
    DOWNLOADING = "DOWNLOADING"
    NOT_FOUND = "NOT_FOUND"
    PARTIAL = "PARTIAL"

class DownloadProgress(BaseModel):
    start_time: float
    elapsed_seconds: float

class ModelStatusResponse(BaseModel):
    model: Model
    status: ModelStatus
    progress: Optional[DownloadProgress] = None
    error_message: Optional[str] = None
```

### 2. ModelManager (`packages/api/src/api/models/manager.py`)

Key methods:
- `is_model_exist(model: Model) -> bool` - Uses `snapshot_download(..., local_files_only=True)` to validate checksums
- `add_model(model: Model) -> str` - Start async download task, return task_id
- `get_model_status(model: Model) -> ModelStatusResponse`
- `cancel_download(model: Model)` - Cancel task and clean partial cache
- `delete_model(model: Model)` - Remove from cache
- `list_models() -> List[Model]` - Enumerate cached models
- `get_disk_space() -> DiskSpaceInfo` - Cache usage and available space

Implementation notes:
- Use `snapshot_download()` for downloading AND verification (with `local_files_only=True` for checking)
- Use `scan_cache_dir()` only for listing and deletion
- Track downloads in `self._download_tasks: Dict[str, asyncio.Task]`
- Limit concurrent downloads to 3
- Generate unique task ID from `model.hf_repo` + `model.hf_commit`

### 3. REST API Endpoints (`packages/api/src/api/models/routes.py`)

All endpoints with OpenAPI examples in docstrings (for Swagger):

#### POST /api/models/status
- **Body**: `Model` (JSON)
- **Returns**: `ModelStatusResponse`
- **Description**: Check model status with cache verification
- **Example**: 
```json
Request: {"hf_repo": "meta-llama/Llama-2-7b-hf", "hf_commit": "abc123"}
Response: {"model": {...}, "status": "DOWNLOADED", "progress": null}
```

#### POST /api/models/download
- **Body**: `Model` (JSON)
- **Returns**: `{"task_id": "...", "status": "DOWNLOADING"}`
- **Description**: Start non-blocking download task
- **Status Codes**: 202 Accepted, 429 Too Many Requests, 409 Conflict (if already downloading)
- **Example**:
```json
Request: {"hf_repo": "meta-llama/Llama-2-7b-hf"}
Response: {"task_id": "meta-llama/Llama-2-7b-hf:latest", "status": "DOWNLOADING"}
```

#### DELETE /api/models
- **Body**: `Model` (JSON)
- **Returns**: `{"status": "deleted" | "cancelled"}`
- **Description**: Cancel download if in progress, delete from cache otherwise
- **Example**:
```json
Request: {"hf_repo": "meta-llama/Llama-2-7b-hf"}
Response: {"status": "deleted"}
```

#### GET /api/models/list
- **Query params**: None
- **Returns**: `{"models": [ModelListItem, ...]}`
- **Description**: List all cached models with their status (DOWNLOADED or PARTIAL)
- **Example**:
```json
Response: {
  "models": [
    {
      "model": {"hf_repo": "meta-llama/Llama-2-7b-hf", "hf_commit": "abc123"},
      "status": "DOWNLOADED"
    },
    {
      "model": {"hf_repo": "microsoft/phi-2", "hf_commit": "def456"},
      "status": "PARTIAL"
    }
  ]
}
```

#### GET /api/models/space
- **Returns**: `{"cache_size_bytes": int, "available_bytes": int, "cache_path": str}`
- **Description**: Disk space usage and availability
- **Example**:
```json
Response: {"cache_size_bytes": 13958643712, "available_bytes": 500000000000, "cache_path": "/root/.cache/huggingface"}
```

### 4. Integration with App (`packages/api/src/api/app.py`)

- Add `app.state.model_manager = ModelManager()` in lifespan
- Include models router: `app.include_router(models_router, prefix=API_PREFIX, tags=["Models"])`

### 5. Testing Strategy

#### Unit Tests (`packages/api/tests/unit/test_model_manager.py`)
- Mock `huggingface_hub` functions
- Test all ModelManager methods
- Test download concurrency limits
- Test cancellation and cleanup

#### Integration Tests (`packages/api/tests/integration/test_models_api.py`)
- Use small test models or mocked downloads
- Test full API flow: download → status → cancel/delete
- Test concurrent download limits
- Test disk space endpoint
- Keep tests fast (< 30s total)

### 6. OpenAPI Documentation

All endpoints must include:
- Detailed docstrings with `"""` format
- Request/response examples in docstring
- Proper Pydantic models for auto-schema generation
- Status code documentation with `@router.post(..., status_code=202, responses={...})`

## File Structure

```
packages/api/src/api/models/
├── __init__.py
├── types.py          # Pydantic models
├── manager.py        # ModelManager class
└── routes.py         # FastAPI router

packages/api/tests/
├── unit/
│   └── test_model_manager.py
└── integration/
    └── test_models_api.py
```

## Dependencies to Add

- `huggingface_hub` (if not already present)
- Verify `asyncio`, `aiofiles` available

## Implementation Status

- [x] Create Model, ModelStatus, and response types in types.py
- [x] Implement ModelManager with HuggingFace cache integration
- [x] Create REST API routes with OpenAPI documentation
- [x] Integrate ModelManager into FastAPI app
- [x] Write unit tests for ModelManager
- [x] Write integration tests for models API
- [x] Add automatic retry logic with exponential backoff (5 attempts, 1-60s)
- [x] Implement post-download verification with file integrity checking
- [x] Add comprehensive tests for retry and verification logic

## Key Enhancements

### Reliability & Verification

1. **Automatic Retry on Network Errors**
   - Up to 5 retry attempts with exponential backoff (1s, 2s, 4s, 8s, 16s, max 60s)
   - Handles: ConnectionError, Timeout, HfHubHTTPError, OSError
   - Uses `tenacity` library for robust retry logic
   - Logs each retry attempt for debugging
   - Download timeout after 24 hours to prevent infinite hangs

2. **Download Verification with Checksum Validation**
   - `snapshot_download()` validates checksums/ETags during download
   - `resume_download=True` re-validates checksums before resuming
   - Post-download verification using `snapshot_download(..., local_files_only=True)` validates all checksums
   - Detects missing, corrupted, or incomplete files
   - Automatically re-downloads files with checksum mismatches
   - Marks download as ERROR if verification fails
   - Simple, canonical approach using library's built-in verification

3. **Resume Support**
   - Downloads can resume if interrupted
   - Uses `resume_download=True` parameter
   - Reduces bandwidth waste on network issues

4. **Enhanced Error Messages**
   - Error messages include retry attempt count
   - Clear distinction between network vs. other errors
   - Detailed logging throughout process

See `planning/task-1-implementation-summary.md` for complete details.

