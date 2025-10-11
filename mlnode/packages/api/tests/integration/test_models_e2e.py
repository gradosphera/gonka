"""End-to-end integration tests for models API.

These tests use REAL HuggingFace models (tiny test models) and perform
actual downloads, file operations, and cache management. They are slow
but provide true integration testing.

Run with: pytest -v -m "not slow" to skip these tests in fast runs.
Run with: pytest -v -m slow to run only these tests.
"""

import pytest
import time
import tempfile
from pathlib import Path
from fastapi.testclient import TestClient
from api.app import app
from api.models.manager import ModelManager


# Use tiny test models from HuggingFace for fast E2E tests
TINY_MODEL = "hf-internal-testing/tiny-random-BertModel"  # ~1MB
TINY_MODEL_2 = "hf-internal-testing/tiny-random-GPT2Model"  # ~1MB


@pytest.fixture
def temp_cache_dir():
    """Create a temporary cache directory for tests."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield tmpdir


@pytest.fixture
def client_with_temp_cache(temp_cache_dir):
    """Create a test client with a temporary cache directory."""
    # Override the ModelManager's cache_dir
    original_init = ModelManager.__init__
    
    def patched_init(self, cache_dir=None):
        original_init(self, cache_dir=temp_cache_dir)
    
    ModelManager.__init__ = patched_init
    client = TestClient(app)
    
    yield client
    
    # Restore original
    ModelManager.__init__ = original_init


@pytest.mark.slow
@pytest.mark.e2e
def test_e2e_download_real_model(client_with_temp_cache, temp_cache_dir):
    """Test downloading a real tiny model from HuggingFace."""
    client = client_with_temp_cache
    
    model_data = {"hf_repo": TINY_MODEL, "hf_commit": None}
    
    # 1. Check status - should not exist initially
    response = client.post("/api/v1/models/status", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] == "NOT_FOUND"
    
    # 2. Start download
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    data = response.json()
    assert "task_id" in data
    assert data["model"]["hf_repo"] == TINY_MODEL
    
    # 3. Wait for download to complete (tiny model should be fast)
    max_wait = 30  # seconds
    start = time.time()
    while time.time() - start < max_wait:
        response = client.post("/api/v1/models/status", json=model_data)
        status = response.json()["status"]
        
        if status == "DOWNLOADED":
            break
        elif status == "PARTIAL":
            pytest.fail(f"Download failed: {response.json().get('error_message')}")
        
        time.sleep(1)
    
    # 4. Verify download completed
    response = client.post("/api/v1/models/status", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] == "DOWNLOADED"
    
    # 5. Verify files exist in cache
    cache_path = Path(temp_cache_dir)
    # HuggingFace creates hub directory structure
    hub_dir = cache_path / "hub"
    assert hub_dir.exists(), f"Cache directory {hub_dir} should exist"
    
    # 6. Verify model appears in list
    response = client.get("/api/v1/models/list")
    assert response.status_code == 200
    models = response.json()["models"]
    assert len(models) > 0
    assert any(m["model"]["hf_repo"] == TINY_MODEL and m["status"] == "DOWNLOADED" for m in models)
    
    # 7. Delete the model
    response = client.request("DELETE", "/api/v1/models", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] == "deleted"
    
    # 8. Verify model is gone
    response = client.post("/api/v1/models/status", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] == "NOT_FOUND"


@pytest.mark.slow
@pytest.mark.e2e
def test_e2e_download_already_exists(client_with_temp_cache):
    """Test downloading a model that already exists."""
    client = client_with_temp_cache
    
    model_data = {"hf_repo": TINY_MODEL, "hf_commit": None}
    
    # 1. Download model
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    
    # 2. Wait for completion
    max_wait = 30
    start = time.time()
    while time.time() - start < max_wait:
        response = client.post("/api/v1/models/status", json=model_data)
        if response.json()["status"] == "DOWNLOADED":
            break
        time.sleep(1)
    
    # 3. Try to download again - should return immediately as DOWNLOADED
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    assert response.json()["status"] == "DOWNLOADED"


@pytest.mark.slow
@pytest.mark.e2e
def test_e2e_concurrent_downloads(client_with_temp_cache):
    """Test concurrent downloads of different models."""
    client = client_with_temp_cache
    
    model1_data = {"hf_repo": TINY_MODEL, "hf_commit": None}
    model2_data = {"hf_repo": TINY_MODEL_2, "hf_commit": None}
    
    # Start both downloads
    response1 = client.post("/api/v1/models/download", json=model1_data)
    assert response1.status_code == 202
    
    response2 = client.post("/api/v1/models/download", json=model2_data)
    assert response2.status_code == 202
    
    # Wait for both to complete
    max_wait = 60  # More time for 2 models
    start = time.time()
    
    while time.time() - start < max_wait:
        status1 = client.post("/api/v1/models/status", json=model1_data).json()["status"]
        status2 = client.post("/api/v1/models/status", json=model2_data).json()["status"]
        
        if status1 == "DOWNLOADED" and status2 == "DOWNLOADED":
            break
        
        if status1 == "PARTIAL" or status2 == "PARTIAL":
            pytest.fail("One of the downloads failed")
        
        time.sleep(2)
    
    # Verify both are downloaded
    response = client.get("/api/v1/models/list")
    models = response.json()["models"]
    assert any(m["model"]["hf_repo"] == TINY_MODEL and m["status"] == "DOWNLOADED" for m in models)
    assert any(m["model"]["hf_repo"] == TINY_MODEL_2 and m["status"] == "DOWNLOADED" for m in models)


@pytest.mark.slow
@pytest.mark.e2e
def test_e2e_cancel_download(client_with_temp_cache):
    """Test cancelling a download in progress."""
    client = client_with_temp_cache
    
    model_data = {"hf_repo": TINY_MODEL, "hf_commit": None}
    
    # Start download
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    
    # Immediately try to cancel (might already be done for tiny model, but that's ok)
    response = client.request("DELETE", "/api/v1/models", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] in ["cancelled", "deleted"]


@pytest.mark.slow
@pytest.mark.e2e
def test_e2e_disk_space(client_with_temp_cache, temp_cache_dir):
    """Test disk space reporting with real cache."""
    client = client_with_temp_cache
    
    # Get initial disk space
    response = client.get("/api/v1/models/space")
    assert response.status_code == 200
    data = response.json()
    assert "cache_size_gb" in data
    assert "available_gb" in data
    assert "cache_path" in data
    assert data["cache_path"] == temp_cache_dir
    
    initial_cache_size = data["cache_size_gb"]
    
    # Download a model
    model_data = {"hf_repo": TINY_MODEL, "hf_commit": None}
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    
    # Wait for download
    max_wait = 30
    start = time.time()
    while time.time() - start < max_wait:
        response = client.post("/api/v1/models/status", json=model_data)
        if response.json()["status"] == "DOWNLOADED":
            break
        time.sleep(1)
    
    # Check disk space again - should have increased
    response = client.get("/api/v1/models/space")
    assert response.status_code == 200
    new_cache_size = response.json()["cache_size_gb"]
    
    # Cache should have grown (tiny model is ~1MB)
    assert new_cache_size > initial_cache_size


@pytest.mark.slow
@pytest.mark.e2e
def test_e2e_invalid_model(client_with_temp_cache):
    """Test attempting to download a non-existent model."""
    client = client_with_temp_cache
    
    model_data = {"hf_repo": "this-model/does-not-exist-12345", "hf_commit": None}
    
    # Start download
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    
    # Wait a bit and check status - should be PARTIAL (failed download)
    time.sleep(5)
    
    response = client.post("/api/v1/models/status", json=model_data)
    data = response.json()
    # Should either be PARTIAL or NOT_FOUND (depending on when we check)
    assert data["status"] in ["PARTIAL", "NOT_FOUND", "DOWNLOADING"]
    
    # If PARTIAL, should have an error message
    if data["status"] == "PARTIAL":
        assert data["error_message"] is not None
        assert len(data["error_message"]) > 0

