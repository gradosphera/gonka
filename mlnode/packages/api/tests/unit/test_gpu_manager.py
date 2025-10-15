import pytest
from unittest.mock import Mock, patch, MagicMock
from api.gpu.manager import GPUManager
from api.gpu.types import GPUDevice, DriverInfo


class TestGPUManager:
    """Unit tests for GPUManager with mocked pynvml."""

    @patch("api.gpu.manager.pynvml")
    def test_init_success(self, mock_pynvml):
        """Test successful NVML initialization."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 2
        
        manager = GPUManager()
        
        assert manager._nvml_initialized is True
        mock_pynvml.nvmlInit.assert_called_once()
        mock_pynvml.nvmlDeviceGetCount.assert_called_once()

    @patch("api.gpu.manager.pynvml")
    def test_init_nvml_error(self, mock_pynvml):
        """Test NVML initialization failure (e.g., no GPU driver)."""
        mock_pynvml.nvmlInit.side_effect = Exception("NVML Shared Library Not Found")
        
        with patch("api.gpu.manager.logger") as mock_logger:
            manager = GPUManager()
            
            assert manager._nvml_initialized is False
            mock_logger.warning.assert_called()

    @patch("api.gpu.manager.pynvml")
    def test_is_cuda_available_true(self, mock_pynvml):
        """Test is_cuda_available returns True when NVML initialized."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        
        manager = GPUManager()
        
        assert manager.is_cuda_available() is True

    @patch("api.gpu.manager.pynvml")
    def test_is_cuda_available_false(self, mock_pynvml):
        """Test is_cuda_available returns False when NVML not initialized."""
        mock_pynvml.nvmlInit.side_effect = Exception("Error")
        
        manager = GPUManager()
        
        assert manager.is_cuda_available() is False

    @patch("api.gpu.manager.pynvml")
    def test_get_devices_no_nvml(self, mock_pynvml):
        """Test get_devices returns empty list when NVML not initialized."""
        mock_pynvml.nvmlInit.side_effect = Exception("Error")
        
        manager = GPUManager()
        devices = manager.get_devices()
        
        assert devices == []

    @patch("api.gpu.manager.pynvml")
    def test_get_devices_success(self, mock_pynvml):
        """Test successful device enumeration with full metrics."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        
        # Mock GPU handle and properties
        mock_handle = Mock()
        mock_pynvml.nvmlDeviceGetHandleByIndex.return_value = mock_handle
        mock_pynvml.nvmlDeviceGetName.return_value = "NVIDIA A100-SXM4-40GB"
        
        # Mock memory info
        mock_mem_info = Mock()
        mock_mem_info.total = 40960 * 1024 * 1024  # 40GB in bytes
        mock_mem_info.free = 35000 * 1024 * 1024
        mock_mem_info.used = 5960 * 1024 * 1024
        mock_pynvml.nvmlDeviceGetMemoryInfo.return_value = mock_mem_info
        
        # Mock utilization
        mock_utilization = Mock()
        mock_utilization.gpu = 45
        mock_pynvml.nvmlDeviceGetUtilizationRates.return_value = mock_utilization
        
        # Mock temperature
        mock_pynvml.nvmlDeviceGetTemperature.return_value = 52
        mock_pynvml.NVML_TEMPERATURE_GPU = 0
        
        manager = GPUManager()
        devices = manager.get_devices()
        
        assert len(devices) == 1
        device = devices[0]
        assert device.index == 0
        assert device.name == "NVIDIA A100-SXM4-40GB"
        assert device.total_memory_mb == 40960
        assert device.free_memory_mb == 35000
        assert device.used_memory_mb == 5960
        assert device.utilization_percent == 45
        assert device.temperature_c == 52
        assert device.is_available is True
        assert device.error_message is None

    @patch("api.gpu.manager.pynvml")
    def test_get_devices_no_gpus(self, mock_pynvml):
        """Test get_devices returns empty list when no GPUs present."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 0
        
        manager = GPUManager()
        devices = manager.get_devices()
        
        assert devices == []

    @patch("api.gpu.manager.pynvml")
    def test_get_devices_partial_metrics_failure(self, mock_pynvml):
        """Test device enumeration with partial metrics failure."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        
        mock_handle = Mock()
        mock_pynvml.nvmlDeviceGetHandleByIndex.return_value = mock_handle
        mock_pynvml.nvmlDeviceGetName.return_value = "NVIDIA RTX 3090"
        
        # Memory succeeds
        mock_mem_info = Mock()
        mock_mem_info.total = 24576 * 1024 * 1024
        mock_mem_info.free = 20000 * 1024 * 1024
        mock_mem_info.used = 4576 * 1024 * 1024
        mock_pynvml.nvmlDeviceGetMemoryInfo.return_value = mock_mem_info
        
        # Utilization fails
        mock_pynvml.nvmlDeviceGetUtilizationRates.side_effect = Exception("Not Supported")
        
        # Temperature fails
        mock_pynvml.nvmlDeviceGetTemperature.side_effect = Exception("Not Supported")
        mock_pynvml.NVML_TEMPERATURE_GPU = 0
        
        manager = GPUManager()
        devices = manager.get_devices()
        
        assert len(devices) == 1
        device = devices[0]
        assert device.index == 0
        assert device.name == "NVIDIA RTX 3090"
        assert device.total_memory_mb == 24576
        assert device.utilization_percent is None
        assert device.temperature_c is None
        assert device.is_available is True

    @patch("api.gpu.manager.pynvml")
    def test_get_devices_device_error(self, mock_pynvml):
        """Test device enumeration with device in error state."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        
        # First device query fails completely
        mock_pynvml.nvmlDeviceGetHandleByIndex.side_effect = Exception("Unknown Error")
        
        manager = GPUManager()
        devices = manager.get_devices()
        
        assert len(devices) == 1
        device = devices[0]
        assert device.index == 0
        assert device.name == "Unknown"
        assert device.is_available is False
        assert device.error_message == "Unknown Error"
        assert device.total_memory_mb is None
        assert device.utilization_percent is None

    @patch("api.gpu.manager.pynvml")
    def test_get_devices_multiple_gpus(self, mock_pynvml):
        """Test enumeration of multiple GPUs."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 2
        
        # Setup mocks for two devices
        mock_handle_0 = Mock()
        mock_handle_0.device_index = 0
        mock_handle_1 = Mock()
        mock_handle_1.device_index = 1
        
        mock_pynvml.nvmlDeviceGetHandleByIndex.side_effect = [mock_handle_0, mock_handle_1]
        mock_pynvml.nvmlDeviceGetName.side_effect = ["NVIDIA A100", "NVIDIA RTX 3090"]
        
        mock_mem_info = Mock()
        mock_mem_info.total = 40960 * 1024 * 1024
        mock_mem_info.free = 35000 * 1024 * 1024
        mock_mem_info.used = 5960 * 1024 * 1024
        mock_pynvml.nvmlDeviceGetMemoryInfo.return_value = mock_mem_info
        
        mock_utilization = Mock()
        mock_utilization.gpu = 50
        mock_pynvml.nvmlDeviceGetUtilizationRates.return_value = mock_utilization
        
        mock_pynvml.nvmlDeviceGetTemperature.return_value = 60
        mock_pynvml.NVML_TEMPERATURE_GPU = 0
        
        manager = GPUManager()
        devices = manager.get_devices()
        
        assert len(devices) == 2
        assert devices[0].index == 0
        assert devices[0].name == "NVIDIA A100"
        assert devices[1].index == 1
        assert devices[1].name == "NVIDIA RTX 3090"

    @patch("api.gpu.manager.pynvml")
    def test_get_driver_info_success(self, mock_pynvml):
        """Test successful driver info retrieval."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        mock_pynvml.nvmlSystemGetDriverVersion.return_value = "535.104.05"
        mock_pynvml.nvmlSystemGetCudaDriverVersion.return_value = 12020  # CUDA 12.2
        mock_pynvml.nvmlSystemGetNVMLVersion.return_value = "12.535.104"
        
        manager = GPUManager()
        driver_info = manager.get_driver_info()
        
        assert isinstance(driver_info, DriverInfo)
        assert driver_info.driver_version == "535.104.05"
        assert driver_info.cuda_driver_version == "12.2"
        assert driver_info.nvml_version == "12.535.104"

    @patch("api.gpu.manager.pynvml")
    def test_get_driver_info_cuda_version_formatting(self, mock_pynvml):
        """Test CUDA version formatting from integer to string."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        mock_pynvml.nvmlSystemGetDriverVersion.return_value = "550.54.15"
        mock_pynvml.nvmlSystemGetCudaDriverVersion.return_value = 12040  # CUDA 12.4
        mock_pynvml.nvmlSystemGetNVMLVersion.return_value = "12.550.54"
        
        manager = GPUManager()
        driver_info = manager.get_driver_info()
        
        assert driver_info.cuda_driver_version == "12.4"

    @patch("api.gpu.manager.pynvml")
    def test_get_driver_info_no_nvml(self, mock_pynvml):
        """Test get_driver_info raises error when NVML not initialized."""
        mock_pynvml.nvmlInit.side_effect = Exception("Error")
        
        manager = GPUManager()
        
        with pytest.raises(RuntimeError, match="NVML not initialized"):
            manager.get_driver_info()

    @patch("api.gpu.manager.pynvml")
    def test_get_driver_info_query_error(self, mock_pynvml):
        """Test get_driver_info handles query errors."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        mock_pynvml.nvmlSystemGetDriverVersion.side_effect = Exception("Query failed")
        
        manager = GPUManager()
        
        with pytest.raises(RuntimeError, match="Failed to query driver information"):
            manager.get_driver_info()

    @patch("api.gpu.manager.pynvml")
    def test_shutdown_nvml_success(self, mock_pynvml):
        """Test successful NVML shutdown."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        
        manager = GPUManager()
        manager._shutdown_nvml()
        
        mock_pynvml.nvmlShutdown.assert_called_once()

    @patch("api.gpu.manager.pynvml")
    def test_shutdown_nvml_not_initialized(self, mock_pynvml):
        """Test shutdown when NVML not initialized doesn't call nvmlShutdown."""
        mock_pynvml.nvmlInit.side_effect = Exception("Error")
        
        manager = GPUManager()
        manager._shutdown_nvml()
        
        mock_pynvml.nvmlShutdown.assert_not_called()

    @patch("api.gpu.manager.pynvml")
    def test_shutdown_nvml_error(self, mock_pynvml):
        """Test shutdown handles errors gracefully."""
        mock_pynvml.nvmlDeviceGetCount.return_value = 1
        mock_pynvml.nvmlShutdown.side_effect = Exception("Shutdown error")
        
        with patch("api.gpu.manager.logger") as mock_logger:
            manager = GPUManager()
            manager._shutdown_nvml()
            
            mock_logger.error.assert_called()

