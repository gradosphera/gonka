import pytest
from fastapi.testclient import TestClient
from api.app import app
from api.gpu.types import GPUDevicesResponse, DriverInfo


class TestGPURoutes:
    """
    Integration tests for GPU routes (no mocking).
    Tests work on both GPU and non-GPU systems.
    """

    @pytest.fixture
    def client(self):
        """Create test client with lifespan events."""
        with TestClient(app) as test_client:
            yield test_client

    def test_get_gpu_devices_endpoint(self, client):
        """Test /api/v1/gpu/devices endpoint returns valid response."""
        response = client.get("/api/v1/gpu/devices")
        
        # Should always return 200, even without GPUs
        assert response.status_code == 200
        
        # Validate response schema
        data = response.json()
        assert "devices" in data
        assert "count" in data
        assert isinstance(data["devices"], list)
        assert isinstance(data["count"], int)
        assert len(data["devices"]) == data["count"]
        
        # If devices present, validate device schema
        for device in data["devices"]:
            assert "index" in device
            assert "name" in device
            assert "is_available" in device
            assert isinstance(device["index"], int)
            assert isinstance(device["name"], str)
            assert isinstance(device["is_available"], bool)
            
            # If device is available, check metric fields exist
            if device["is_available"]:
                assert device["error_message"] is None
                # Metrics can be None if query failed
                assert "total_memory_mb" in device
                assert "free_memory_mb" in device
                assert "used_memory_mb" in device
                assert "utilization_percent" in device
                assert "temperature_c" in device
            else:
                # If not available, should have error message
                assert device["error_message"] is not None

    def test_get_gpu_devices_validates_against_pydantic(self, client):
        """Test response can be parsed by Pydantic model."""
        response = client.get("/api/v1/gpu/devices")
        assert response.status_code == 200
        
        # Parse with Pydantic model to validate schema
        devices_response = GPUDevicesResponse(**response.json())
        assert isinstance(devices_response, GPUDevicesResponse)
        assert devices_response.count == len(devices_response.devices)

    def test_get_driver_info_endpoint_with_gpu(self, client):
        """
        Test /api/v1/gpu/driver endpoint.
        
        Expected behavior:
        - Returns 200 with driver info if GPU/NVML available
        - Returns 503 if NVML not initialized
        """
        response = client.get("/api/v1/gpu/driver")
        
        # Should return either 200 (GPU available) or 503 (no GPU/NVML)
        assert response.status_code in [200, 503]
        
        if response.status_code == 200:
            # Validate response schema
            data = response.json()
            assert "driver_version" in data
            assert "cuda_driver_version" in data
            assert "nvml_version" in data
            assert isinstance(data["driver_version"], str)
            assert isinstance(data["cuda_driver_version"], str)
            assert isinstance(data["nvml_version"], str)
            
            # Parse with Pydantic model
            driver_info = DriverInfo(**data)
            assert isinstance(driver_info, DriverInfo)
            
            # Check version format (basic sanity check)
            assert len(driver_info.driver_version) > 0
            assert "." in driver_info.cuda_driver_version  # e.g., "12.2"
        else:
            # 503 response should have error detail
            assert "detail" in response.json()

    def test_get_driver_info_validates_against_pydantic(self, client):
        """Test driver info response can be parsed by Pydantic model."""
        response = client.get("/api/v1/gpu/driver")
        
        if response.status_code == 200:
            driver_info = DriverInfo(**response.json())
            assert isinstance(driver_info, DriverInfo)

    def test_devices_endpoint_with_no_gpu_returns_empty_list(self, client):
        """
        Test that devices endpoint returns empty list gracefully on non-GPU systems.
        
        Note: This test will pass on GPU systems too (just with non-empty list).
        """
        response = client.get("/api/v1/gpu/devices")
        assert response.status_code == 200
        
        data = response.json()
        # On non-GPU systems, should get empty list
        # On GPU systems, should get list with devices
        assert isinstance(data["devices"], list)
        assert data["count"] >= 0

    def test_gpu_endpoints_in_openapi_schema(self, client):
        """Test that GPU endpoints are included in OpenAPI schema."""
        response = client.get("/openapi.json")
        assert response.status_code == 200
        
        openapi_schema = response.json()
        paths = openapi_schema.get("paths", {})
        
        # Check GPU endpoints are documented
        assert "/api/v1/gpu/devices" in paths
        assert "/api/v1/gpu/driver" in paths
        
        # Check devices endpoint
        devices_path = paths["/api/v1/gpu/devices"]
        assert "get" in devices_path
        assert "tags" in devices_path["get"]
        assert "GPU" in devices_path["get"]["tags"]
        
        # Check driver endpoint
        driver_path = paths["/api/v1/gpu/driver"]
        assert "get" in driver_path
        assert "tags" in driver_path["get"]
        assert "GPU" in driver_path["get"]["tags"]

    def test_devices_response_structure_consistency(self, client):
        """Test that multiple calls to devices endpoint return consistent structure."""
        response1 = client.get("/api/v1/gpu/devices")
        response2 = client.get("/api/v1/gpu/devices")
        
        assert response1.status_code == 200
        assert response2.status_code == 200
        
        data1 = response1.json()
        data2 = response2.json()
        
        # Device count should be stable
        assert data1["count"] == data2["count"]
        
        # Device indices should match
        if data1["count"] > 0:
            indices1 = [d["index"] for d in data1["devices"]]
            indices2 = [d["index"] for d in data2["devices"]]
            assert indices1 == indices2

    def test_driver_info_response_stability(self, client):
        """Test that driver info is stable across multiple calls."""
        response1 = client.get("/api/v1/gpu/driver")
        
        if response1.status_code == 200:
            response2 = client.get("/api/v1/gpu/driver")
            assert response2.status_code == 200
            
            data1 = response1.json()
            data2 = response2.json()
            
            # Driver info should be identical
            assert data1["driver_version"] == data2["driver_version"]
            assert data1["cuda_driver_version"] == data2["cuda_driver_version"]
            assert data1["nvml_version"] == data2["nvml_version"]

