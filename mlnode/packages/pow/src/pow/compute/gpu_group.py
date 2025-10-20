from typing import List, Dict
import torch
from pow.models.utils import PARAMS_V1, PARAMS_V2, Params
from common.logger import create_logger

logger = create_logger(__name__)


class NotEnoughGPUResources(Exception):
    """Raised when GPU is not available or has insufficient resources"""
    pass


def get_min_group_vram(params: Params) -> float:
    if params == PARAMS_V1:
        return 10.0
    elif params == PARAMS_V2:
        return 38.0
    else:
        return 38.0

class GpuGroup:
    def __init__(self, devices: List[int]):
        if not devices:
            raise ValueError("GPU group must have at least one device")
        
        self.devices = devices
        self.primary_device = devices[0]  # First device is primary
        self.group_size = len(devices)
    
    def __repr__(self):
        return f"GpuGroup(devices={self.devices}, primary={self.primary_device})"
    
    def get_device_strings(self) -> List[str]:
        return [f"cuda:{device_id}" for device_id in self.devices]
    
    def get_primary_device_string(self) -> str:
        return f"cuda:{self.primary_device}"
    
    def get_total_vram_gb(self) -> float:
        if not torch.cuda.is_available():
            return 0.0
            
        total_vram = 0.0
        for device_id in self.devices:
            if device_id < torch.cuda.device_count():
                props = torch.cuda.get_device_properties(device_id)
                total_vram += props.total_memory / (1024**3)  # Convert to GB
        return total_vram

    def get_free_vram_mb_per_device(self) -> Dict[int, int]:
        if not torch.cuda.is_available():
            return {device_id: 0 for device_id in self.devices}
        
        free_vram_map = {}
        for device_id in self.devices:
            if device_id < torch.cuda.device_count():
                free_mem_bytes, _ = torch.cuda.mem_get_info(device_id)
                free_vram_map[device_id] = int(free_mem_bytes / (1024**2))
            else:
                free_vram_map[device_id] = 0
        return free_vram_map

    def get_free_vram_gb(self) -> float:
        free_vram_per_device_mb = self.get_free_vram_mb_per_device()
        
        total_free_vram_mb = sum(free_vram_per_device_mb.values())
        
        return total_free_vram_mb / 1024

def create_gpu_groups(min_vram_gb: float = None, params: Params = None) -> List[GpuGroup]:

    if not torch.cuda.is_available():
        error_msg = "CUDA is not available - no GPU support detected"
        logger.error(error_msg)
        raise NotEnoughGPUResources(error_msg)

    if min_vram_gb is None:
        min_vram_gb = get_min_group_vram(params)

    device_count = torch.cuda.device_count()
    if device_count == 0:
        error_msg = "No CUDA devices found - GPU count is 0"
        logger.error(error_msg)
        raise NotEnoughGPUResources(error_msg)

    # Get VRAM for each device, sorted by device_id for determinism
    device_vram = []
    for device_id in range(device_count):
        props = torch.cuda.get_device_properties(device_id)
        vram_gb = props.total_memory / (1024**3)
        device_vram.append((device_id, vram_gb))

    groups = []
    available_devices = list(device_vram)
    preferred_sizes = [1, 2, 4, 8]

    while available_devices:
        group_formed = False
        for group_size in preferred_sizes:
            if len(available_devices) >= group_size:
                potential_group_tuples = available_devices[:group_size]
                total_vram = sum(vram for _, vram in potential_group_tuples)

                if total_vram >= min_vram_gb:
                    device_ids = [device_id for device_id, _ in potential_group_tuples]
                    groups.append(GpuGroup(device_ids))
                    available_devices = available_devices[group_size:]
                    group_formed = True
                    break  # Found a valid group, move to next block of available devices
        
        if not group_formed:
            # Could not form a valid group starting with the current device.
            # Discard it and try to form a group from the remaining devices.
            discarded_device = available_devices.pop(0)
            logger.warning(f"GPU {discarded_device[0]} has insufficient VRAM ({discarded_device[1]:.1f}GB) to form a group, required: {min_vram_gb:.1f}GB")

    if not groups:
        device_info = ", ".join([f"GPU{device_id}: {vram:.1f}GB" for device_id, vram in device_vram])
        error_msg = f"Not enough GPU memory to form any groups - required: {min_vram_gb:.1f}GB per group, available: [{device_info}]"
        logger.error(error_msg)
        raise NotEnoughGPUResources(error_msg)

    return groups
