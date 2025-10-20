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

