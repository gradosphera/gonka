# GPU Management System Implementation Plan

## Overview

Implement a minimalistic `GPUManager` class with REST API for monitoring CUDA GPU devices and driver information. Read-only monitoring using `pynvml` for reliable, direct access to NVIDIA GPU metrics.

## Architecture Decisions

### GPU Detection Library

- Use **pynvml** (official NVIDIA Management Library Python bindings)
- Direct access to NVIDIA driver API (same as nvidia-smi uses)
- No subprocess parsing, structured data with proper error handling
- Minimal dependency (~100KB), standard on GPU systems

### API Design

- **Read-only monitoring** (no GPU management operations for MVP)
- Simple GET endpoints returning structured JSON
- No state tracking needed (queries NVIDIA driver on-demand)
- Singleton GPUManager attached to app.state

### Error Handling

- Graceful handling when no GPUs present
- Return empty device list on systems without CUDA
- Clear error messages for driver/library issues

## Implementation Details

### 1. Core GPU Types (`packages/api/src/api/gpu/types.py`)

```python
from pydantic import BaseModel
from typing import List, Optional

class GPUDevice(BaseModel):
    index: int
    name: str  # GPU type (e.g., "NVIDIA A100-SXM4-40GB")
    total_memory_mb: Optional[int] = None  # None if GPU in error state
    free_memory_mb: Optional[int] = None
    used_memory_mb: Optional[int] = None
    utilization_percent: Optional[int] = None  # GPU compute utilization
    temperature_c: Optional[int] = None
    is_available: bool  # Can query device successfully
    error_message: Optional[str] = None  # Error details if is_available=False

class GPUDevicesResponse(BaseModel):
    devices: List[GPUDevice]
    count: int

class DriverInfo(BaseModel):
    driver_version: str  # e.g., "535.104.05"
    cuda_driver_version: str  # Max CUDA supported by driver (e.g., "12.2")
    nvml_version: str  # NVML library version
```

### 2. GPUManager (`packages/api/src/api/gpu/manager.py`)

Key methods:

- `get_devices() -> List[GPUDevice]` - Query all GPU devices with current metrics
- `get_driver_info() -> DriverInfo` - Get CUDA driver information
- `is_cuda_available() -> bool` - Check if CUDA is available
- `_init_nvml()` - Initialize pynvml (called in constructor)
- `_shutdown_nvml()` - Cleanup pynvml (called on app shutdown)

Implementation notes:

- Initialize pynvml once in constructor, track success with `_nvml_initialized` flag
- Only call `nvmlShutdown()` if initialization succeeded
- Query device info on-demand (no caching, always fresh metrics)
- Graceful error handling: catch exceptions per-GPU, populate `error_message` field
- Return empty list if NVML not initialized or no GPUs detected
- Multi-process safe: each process maintains its own NVML state
- Add logging for initialization, errors, and device queries

### 3. REST API Endpoints (`packages/api/src/api/gpu/routes.py`)

#### GET /api/gpu/devices

- **Returns**: `GPUDevicesResponse` (HTTP 200)
- **Description**: List all CUDA devices with current metrics
- **Behavior**: Returns empty list if no GPUs or NVML not initialized
- **Example with GPU**:
```json
{
  "devices": [
    {
      "index": 0,
      "name": "NVIDIA A100-SXM4-40GB",
      "total_memory_mb": 40960,
      "free_memory_mb": 35000,
      "used_memory_mb": 5960,
      "utilization_percent": 45,
      "temperature_c": 52,
      "is_available": true,
      "error_message": null
    }
  ],
  "count": 1
}
```

- **Example without GPU**:
```json
{
  "devices": [],
  "count": 0
}
```

#### GET /api/gpu/driver

- **Returns**: `DriverInfo` (HTTP 200)
- **Description**: CUDA driver version information from NVML
- **Note**: `cuda_driver_version` is the maximum CUDA version supported by the installed NVIDIA driver, not the CUDA toolkit version
- **Example**:
```json
{
  "driver_version": "535.104.05",
  "cuda_driver_version": "12.2",
  "nvml_version": "12.535.104"
}
```


### 4. Integration with App (`packages/api/src/api/app.py`)

- Add `app.state.gpu_manager = GPUManager()` in lifespan startup
- Call `gpu_manager._shutdown_nvml()` in lifespan shutdown
- Include GPU router: `app.include_router(gpu_router, prefix=API_PREFIX + "/gpu", tags=["GPU"])`

### 5. Testing Strategy

#### Unit Tests (`packages/api/tests/unit/test_gpu_manager.py`)

- Mock pynvml functions to test logic without hardware dependency
- Test device enumeration with mocked GPU data
- Test driver info retrieval
- Test error handling (no GPUs, NVML init failures, per-device errors)
- Test `is_available` and `error_message` logic
- Test `_nvml_initialized` flag behavior
- Verify graceful degradation (empty lists when NVML unavailable)
- Fast, deterministic tests that run in any CI environment

#### E2E Integration Tests (`packages/api/tests/integration/test_gpu_routes.py`)

- No mocking - test against real NVML library
- Test `/api/gpu/devices` endpoint
- Test `/api/gpu/driver` endpoint  
- Verify response schemas match Pydantic models
- Tests are system-dependent:
  - On GPU systems: verify device data populated
  - On non-GPU systems: verify empty list returned gracefully
  - Both scenarios return HTTP 200
- Validates observability works in production-like conditions

### 6. OpenAPI Documentation

All endpoints include:

- Detailed docstrings with examples
- Proper Pydantic models for auto-schema
- Clear response schemas in Swagger UI

## File Structure

```
packages/api/src/api/gpu/
├── __init__.py
├── types.py          # Pydantic models
├── manager.py        # GPUManager class
└── routes.py         # FastAPI router

packages/api/tests/
├── unit/
│   └── test_gpu_manager.py   # Unit tests (mocked pynvml)
└── integration/
    └── test_gpu_routes.py    # E2E tests (no mocking)
```

## Dependencies to Add

- `nvidia-ml-py` (pynvml) - official NVIDIA Python bindings

## Logging Strategy

Add structured logging in `manager.py`:

- **INFO**: NVML initialization success with GPU count
- **WARNING**: NVML initialization failed (no GPUs/driver)
- **ERROR**: Errors querying GPU devices or driver info
- **DEBUG**: Empty device list returns

Example log messages:
```
INFO: NVML initialized successfully. Found 2 GPU(s)
WARNING: NVML initialization failed: NVML Shared Library Not Found. GPU features disabled.
ERROR: Error querying GPU device 0: Unknown Error
DEBUG: NVML not initialized, returning empty device list
```

## Future Enhancements (NOT IMPLEMENT NOW)

### Container-Safe GPU Management Operations

These operations can potentially be added in the future:

**Safe Operations (can run in container with proper privileges):**

- GPU device reset (if container has CAP_SYS_ADMIN or --privileged)
- Set persistence mode (keeps driver loaded, faster cold starts)
- Clear ECC error counters

**Operations requiring host-level access (typically not container-safe):**

- Change compute mode (exclusive process, prohibited, default)
- Change power limit
- Enable/disable ECC memory
- Driver updates

**Recommendation for MVP**: Keep read-only. Management operations require:

- Container must run with `--privileged` or specific capabilities
- Shared /dev/nvidia* devices with host
- May conflict with other containers using GPUs
- Better handled at orchestration layer (k8s device plugins, Docker runtime)

If management features are needed later, implement with:

- Feature flag to enable management operations
- Validation of container privileges before attempting operations
- Clear error messages when insufficient permissions
- Separate `/api/gpu/manage/*` endpoints with auth requirements