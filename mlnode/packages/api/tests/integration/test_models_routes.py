"""Route/Controller tests for models API.

These tests verify HTTP routes, request/response serialization, status codes,
and error handling with mocked dependencies. They are fast but don't test
actual HuggingFace downloads or real file system operations.

For true end-to-end integration tests, see test_models_e2e.py.
"""

import pytest
from fastapi.testclient import TestClient
from unittest.mock import Mock, patch, MagicMock
from api.app import app
from api.models.types import ModelStatus


class MockCacheInfo:
    """Mock for HuggingFace cache info."""
    
    def __init__(self, repos=None):
        self.repos = repos or []
        self.size_on_disk = 1000000
    
    def delete_revisions(self, *args):
        """Mock delete_revisions."""
        mock_strategy = Mock()
        mock_strategy.expected_freed_size_str = "1.0 GB"
        mock_strategy.execute = Mock()
        return mock_strategy


class MockRevision:
    """Mock for HuggingFace revision."""
    
    def __init__(self, commit_hash):
        self.commit_hash = commit_hash


class MockRepo:
    """Mock for HuggingFace repo."""
    
    def __init__(self, repo_id, revisions=None):
        self.repo_id = repo_id
        self.revisions = revisions or []


@pytest.fixture
def client():
    """Create a test client."""
    return TestClient(app)


@pytest.fixture
def sample_model_data():
    """Sample model data for requests."""
    return {
        "hf_repo": "test/model",
        "hf_commit": "abc123"
    }


@pytest.fixture
def sample_model_data_no_commit():
    """Sample model data without commit."""
    return {
        "hf_repo": "test/model",
        "hf_commit": None
    }


@patch('api.models.manager.snapshot_download')
def test_check_model_status_not_found(mock_snapshot, client, sample_model_data):
    """Test checking status of non-existent model."""
    from huggingface_hub.utils import RepositoryNotFoundError
    mock_snapshot.side_effect = RepositoryNotFoundError("Not found")
    
    response = client.post("/api/v1/models/status", json=sample_model_data)
    
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "NOT_FOUND"
    assert data["model"]["hf_repo"] == "test/model"


@patch('api.models.manager.snapshot_download')
def test_check_model_status_downloaded(mock_snapshot, client, sample_model_data):
    """Test checking status of downloaded model."""
    # Model exists and is valid
    mock_snapshot.return_value = "/tmp/test_cache"
    
    response = client.post("/api/v1/models/status", json=sample_model_data)
    
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "DOWNLOADED"
    assert data["model"]["hf_repo"] == "test/model"
    assert data["progress"] is None


@patch('api.models.manager.snapshot_download')
def test_download_model(mock_snapshot, client, sample_model_data):
    """Test starting model download."""
    # Model doesn't exist, then gets downloaded
    from huggingface_hub.utils import RepositoryNotFoundError
    mock_snapshot.side_effect = [
        RepositoryNotFoundError("Not found"),  # is_model_exist check
        "/tmp/test_cache",  # download
        "/tmp/test_cache",  # verification
    ]
    
    response = client.post("/api/v1/models/download", json=sample_model_data)
    
    assert response.status_code == 202
    data = response.json()
    assert data["task_id"] == "test/model:abc123"
    assert data["status"] in ["DOWNLOADING", "DOWNLOADED"]
    assert data["model"]["hf_repo"] == "test/model"


@patch('api.models.manager.snapshot_download')
def test_download_model_already_exists(mock_snapshot, client, sample_model_data):
    """Test downloading a model that already exists."""
    # Model already exists
    mock_snapshot.return_value = "/tmp/test_cache"
    
    response = client.post("/api/v1/models/download", json=sample_model_data)
    
    assert response.status_code == 202
    data = response.json()
    assert data["status"] == "DOWNLOADED"


@patch('api.models.manager.snapshot_download')
def test_download_model_already_downloading(mock_snapshot, client, sample_model_data):
    """Test downloading a model that's already downloading."""
    from huggingface_hub.utils import RepositoryNotFoundError
    # Model doesn't exist initially
    mock_snapshot.side_effect = [
        RepositoryNotFoundError("Not found"),  # is_model_exist check for first download
        "/tmp/test_cache",  # download (won't complete before second request)
        RepositoryNotFoundError("Not found"),  # is_model_exist check for second download
    ]
    
    # Start first download
    response1 = client.post("/api/v1/models/download", json=sample_model_data)
    assert response1.status_code == 202
    
    # Try to start second download
    response2 = client.post("/api/v1/models/download", json=sample_model_data)
    assert response2.status_code == 409  # Conflict


@patch('api.models.manager.snapshot_download')
def test_download_max_concurrent(mock_snapshot, client):
    """Test maximum concurrent downloads."""
    from huggingface_hub.utils import RepositoryNotFoundError
    # All models don't exist
    mock_snapshot.side_effect = [RepositoryNotFoundError("Not found")] * 10
    
    # Start 3 downloads
    for i in range(3):
        model_data = {"hf_repo": f"test/model{i}", "hf_commit": None}
        response = client.post("/api/v1/models/download", json=model_data)
        assert response.status_code == 202
    
    # Try to start 4th download
    model_data = {"hf_repo": "test/model4", "hf_commit": None}
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 429  # Too Many Requests


@patch('api.models.manager.scan_cache_dir')
@patch('api.models.manager.snapshot_download')
def test_delete_model(mock_snapshot, mock_scan, client, sample_model_data):
    """Test deleting a model."""
    # Model exists
    mock_snapshot.return_value = "/tmp/test_cache"
    
    revision = MockRevision("abc123")
    repo = MockRepo("test/model", [revision])
    cache_info = MockCacheInfo([repo])
    mock_scan.return_value = cache_info
    
    response = client.request("DELETE", "/api/v1/models", json=sample_model_data)
    
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "deleted"
    assert data["model"]["hf_repo"] == "test/model"


@patch('api.models.manager.snapshot_download')
def test_delete_model_not_found(mock_snapshot, client, sample_model_data):
    """Test deleting non-existent model."""
    from huggingface_hub.utils import RepositoryNotFoundError
    mock_snapshot.side_effect = RepositoryNotFoundError("Not found")
    
    response = client.request("DELETE", "/api/v1/models", json=sample_model_data)
    
    assert response.status_code == 404


@patch('api.models.manager.snapshot_download')
def test_delete_model_downloading(mock_snapshot, client, sample_model_data):
    """Test deleting a model that's downloading cancels it."""
    from huggingface_hub.utils import RepositoryNotFoundError
    mock_snapshot.side_effect = [
        RepositoryNotFoundError("Not found"),  # is_model_exist check
        "/tmp/test_cache",  # download (won't complete)
    ]
    
    # Start download
    response1 = client.post("/api/v1/models/download", json=sample_model_data)
    assert response1.status_code == 202
    
    # Delete/cancel it
    response2 = client.request("DELETE", "/api/v1/models", json=sample_model_data)
    assert response2.status_code == 200
    data = response2.json()
    assert data["status"] == "cancelled"


@patch('api.models.manager.scan_cache_dir')
def test_list_models_empty(mock_scan, client):
    """Test listing models when cache is empty."""
    mock_scan.return_value = MockCacheInfo([])
    
    response = client.get("/api/v1/models/list")
    
    assert response.status_code == 200
    data = response.json()
    assert data["models"] == []


@patch('api.models.manager.scan_cache_dir')
def test_list_models(mock_scan, client):
    """Test listing models."""
    revision1 = MockRevision("abc123")
    revision2 = MockRevision("def456")
    repo1 = MockRepo("test/model1", [revision1])
    repo2 = MockRepo("test/model2", [revision2])
    mock_scan.return_value = MockCacheInfo([repo1, repo2])
    
    response = client.get("/api/v1/models/list")
    
    assert response.status_code == 200
    data = response.json()
    assert len(data["models"]) == 2
    assert any(m["hf_repo"] == "test/model1" for m in data["models"])
    assert any(m["hf_repo"] == "test/model2" for m in data["models"])


@patch('api.models.manager.scan_cache_dir')
@patch('api.models.manager.shutil.disk_usage')
def test_get_disk_space(mock_disk_usage, mock_scan, client):
    """Test getting disk space information."""
    mock_scan.return_value = MockCacheInfo([])
    
    mock_stat = Mock()
    mock_stat.free = 500000000000
    mock_disk_usage.return_value = mock_stat
    
    response = client.get("/api/v1/models/space")
    
    assert response.status_code == 200
    data = response.json()
    assert "cache_size_gb" in data
    assert "available_gb" in data
    assert "cache_path" in data
    # 1000000 bytes = ~0.0 GB (rounds to 0.0)
    assert data["cache_size_gb"] == 0.0
    # 500000000000 bytes = ~465.66 GB
    assert data["available_gb"] == 465.66


@patch('api.models.manager.scan_cache_dir')
@patch('api.models.manager.snapshot_download')
def test_full_workflow(mock_snapshot, mock_scan, client):
    """Test full workflow: check status, download, check again, delete."""
    from huggingface_hub.utils import RepositoryNotFoundError
    
    # 1. Check status - model doesn't exist initially
    model_data = {"hf_repo": "test/workflow", "hf_commit": None}
    mock_snapshot.side_effect = [RepositoryNotFoundError("Not found")]
    response = client.post("/api/v1/models/status", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] == "NOT_FOUND"
    
    # 2. Start download
    mock_snapshot.side_effect = [
        RepositoryNotFoundError("Not found"),  # is_model_exist check
        "/tmp/test_cache",  # download
        "/tmp/test_cache",  # verification
    ]
    response = client.post("/api/v1/models/download", json=model_data)
    assert response.status_code == 202
    task_id = response.json()["task_id"]
    assert task_id == "test/workflow:latest"
    
    # 3. Check status again - model now exists
    mock_snapshot.side_effect = ["/tmp/test_cache"]
    response = client.post("/api/v1/models/status", json=model_data)
    assert response.status_code == 200
    # Status could be DOWNLOADING or DOWNLOADED depending on timing
    assert response.json()["status"] in ["DOWNLOADING", "DOWNLOADED"]
    
    # 4. Delete the model
    # Mock the model as existing in cache for deletion
    revision = MockRevision("latest123")
    repo = MockRepo("test/workflow", [revision])
    mock_scan.return_value = MockCacheInfo([repo])
    mock_snapshot.side_effect = ["/tmp/test_cache"]  # is_model_exist check
    
    response = client.request("DELETE", "/api/v1/models", json=model_data)
    assert response.status_code == 200
    assert response.json()["status"] in ["deleted", "cancelled"]


def test_invalid_model_data(client):
    """Test API with invalid model data."""
    # Missing required field
    response = client.post("/api/v1/models/status", json={"hf_commit": "abc123"})
    assert response.status_code == 422  # Unprocessable Entity
    
    # Empty repo name
    response = client.post("/api/v1/models/status", json={"hf_repo": "", "hf_commit": None})
    assert response.status_code in [200, 422]  # Depending on validation

