"""Unit tests for ModelManager."""

import asyncio
import pytest
from unittest.mock import Mock, patch, MagicMock
import requests
from api.models.manager import ModelManager, DownloadTask
from api.models.types import Model, ModelStatus


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
    
    def __init__(self, commit_hash, num_files=10, size_on_disk=1000000):
        self.commit_hash = commit_hash
        self.files = [f"file{i}.bin" for i in range(num_files)]
        self.size_on_disk = size_on_disk
        self.size_on_disk_str = f"{size_on_disk / (1024**3):.2f} GB"


class MockRepo:
    """Mock for HuggingFace repo."""
    
    def __init__(self, repo_id, revisions=None):
        self.repo_id = repo_id
        self.revisions = revisions or []


@pytest.fixture
def manager():
    """Create a ModelManager instance."""
    return ModelManager(cache_dir="/tmp/test_cache")


@pytest.fixture
def sample_model():
    """Create a sample model."""
    return Model(hf_repo="test/model", hf_commit="abc123")


@pytest.fixture
def sample_model_no_commit():
    """Create a sample model without commit."""
    return Model(hf_repo="test/model")


def test_get_task_id(manager, sample_model, sample_model_no_commit):
    """Test task ID generation."""
    assert manager._get_task_id(sample_model) == "test/model:abc123"
    assert manager._get_task_id(sample_model_no_commit) == "test/model:latest"


@patch('api.models.manager.snapshot_download')
def test_is_model_exist_with_commit(mock_snapshot, manager, sample_model):
    """Test checking if model exists with specific commit."""
    # Mock successful verification (no exception = model exists and is valid)
    mock_snapshot.return_value = "/tmp/test_cache/models/test/model"
    
    assert manager.is_model_exist(sample_model) is True
    mock_snapshot.assert_called_once_with(
        repo_id="test/model",
        revision="abc123",
        cache_dir=manager.cache_dir,
        local_files_only=True,
    )


@patch('api.models.manager.snapshot_download')
def test_is_model_exist_without_commit(mock_snapshot, manager, sample_model_no_commit):
    """Test checking if model exists without specific commit."""
    # Mock successful verification (no exception = model exists and is valid)
    mock_snapshot.return_value = "/tmp/test_cache/models/test/model"
    
    assert manager.is_model_exist(sample_model_no_commit) is True
    mock_snapshot.assert_called_once_with(
        repo_id="test/model",
        revision=None,
        cache_dir=manager.cache_dir,
        local_files_only=True,
    )


@patch('api.models.manager.snapshot_download')
def test_is_model_exist_not_found(mock_snapshot, manager, sample_model):
    """Test checking if model exists when it doesn't."""
    # Mock model not found (raise exception)
    from huggingface_hub.utils import RepositoryNotFoundError
    mock_snapshot.side_effect = RepositoryNotFoundError("Not found")
    
    assert manager.is_model_exist(sample_model) is False


@patch('api.models.manager.snapshot_download')
def test_is_model_exist_wrong_commit(mock_snapshot, manager, sample_model):
    """Test checking if model exists with wrong commit."""
    # Mock cache with different commit (local_files_only will fail)
    from huggingface_hub.utils import RevisionNotFoundError
    mock_snapshot.side_effect = RevisionNotFoundError("Revision not found")
    
    assert manager.is_model_exist(sample_model) is False


@pytest.mark.asyncio
async def test_add_model_already_exists(manager, sample_model):
    """Test adding a model that already exists."""
    with patch.object(manager, 'is_model_exist', return_value=True):
        task_id = await manager.add_model(sample_model)
        
        assert task_id == "test/model:abc123"
        assert task_id in manager._download_tasks
        assert manager._download_tasks[task_id].status == ModelStatus.DOWNLOADED


@pytest.mark.asyncio
async def test_add_model_starts_download(manager, sample_model):
    """Test adding a model starts download."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_download_model', return_value=None) as mock_download:
        
        task_id = await manager.add_model(sample_model)
        
        assert task_id == "test/model:abc123"
        assert task_id in manager._download_tasks
        assert manager._download_tasks[task_id].status == ModelStatus.DOWNLOADING


@pytest.mark.asyncio
async def test_add_model_already_downloading(manager, sample_model):
    """Test adding a model that's already downloading raises error."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_download_model', return_value=None):
        
        # Start first download
        await manager.add_model(sample_model)
        
        # Try to start second download
        with pytest.raises(ValueError, match="already downloading"):
            await manager.add_model(sample_model)


@pytest.mark.asyncio
async def test_add_model_max_concurrent(manager):
    """Test max concurrent downloads limit."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_download_model', return_value=None):
        
        # Start 3 downloads
        for i in range(3):
            model = Model(hf_repo=f"test/model{i}")
            await manager.add_model(model)
        
        # Try to start 4th download
        model4 = Model(hf_repo="test/model4")
        with pytest.raises(ValueError, match="Maximum concurrent downloads"):
            await manager.add_model(model4)


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_model_success(mock_snapshot, manager, sample_model):
    """Test successful model download."""
    mock_snapshot.return_value = "/tmp/test_cache/models/test/model"
    
    task_obj = DownloadTask(sample_model)
    await manager._download_model("test/model:abc123", sample_model, task_obj)
    
    assert task_obj.status == ModelStatus.DOWNLOADED
    assert task_obj.error_message is None


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_model_error(mock_snapshot, manager, sample_model):
    """Test model download with error."""
    mock_snapshot.side_effect = Exception("Network error")
    
    task_obj = DownloadTask(sample_model)
    await manager._download_model("test/model:abc123", sample_model, task_obj)
    
    assert task_obj.status == ModelStatus.PARTIAL
    assert "Network error" in task_obj.error_message


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_model_cancelled(mock_snapshot, manager, sample_model):
    """Test model download cancellation."""
    # Make snapshot_download block so we can cancel it
    # Since it's run in an executor, we need to make it sleep synchronously
    import time as sync_time
    
    def slow_download(*args, **kwargs):
        sync_time.sleep(10)
        return "/tmp/test_cache"
    
    mock_snapshot.side_effect = slow_download
    
    task_obj = DownloadTask(sample_model)
    download_task = asyncio.create_task(
        manager._download_model("test/model:abc123", sample_model, task_obj)
    )
    
    # Give it a moment to start
    await asyncio.sleep(0.1)
    
    # Cancel the task
    download_task.cancel()
    
    with pytest.raises(asyncio.CancelledError):
        await download_task
    
    assert task_obj.status == ModelStatus.PARTIAL


def test_get_model_status_not_found(manager, sample_model):
    """Test getting status for non-existent model."""
    with patch.object(manager, 'is_model_exist', return_value=False):
        status = manager.get_model_status(sample_model)
        
        assert status.model == sample_model
        assert status.status == ModelStatus.NOT_FOUND
        assert status.progress is None


def test_get_model_status_downloaded(manager, sample_model):
    """Test getting status for downloaded model."""
    with patch.object(manager, 'is_model_exist', return_value=True):
        status = manager.get_model_status(sample_model)
        
        assert status.model == sample_model
        assert status.status == ModelStatus.DOWNLOADED
        assert status.progress is None


@pytest.mark.asyncio
async def test_get_model_status_downloading(manager, sample_model):
    """Test getting status for downloading model."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_download_model', return_value=None):
        
        task_id = await manager.add_model(sample_model)
        status = manager.get_model_status(sample_model)
        
        assert status.model == sample_model
        assert status.status == ModelStatus.DOWNLOADING
        assert status.progress is not None


def test_get_model_status_partial(manager, sample_model):
    """Test getting status for model with partial files in cache."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_has_partial_files', return_value=True):
        
        status = manager.get_model_status(sample_model)
        
        assert status.model == sample_model
        assert status.status == ModelStatus.PARTIAL
        assert status.progress is None


def test_has_partial_files_repo_not_in_cache(manager, sample_model):
    """Test _has_partial_files when repo is not in cache."""
    mock_cache_info = MagicMock()
    mock_cache_info.repos = []
    
    with patch('api.models.manager.scan_cache_dir', return_value=mock_cache_info):
        assert manager._has_partial_files(sample_model) is False


def test_has_partial_files_repo_in_cache(manager, sample_model):
    """Test _has_partial_files when repo is in cache."""
    mock_repo = MagicMock()
    mock_repo.repo_id = sample_model.hf_repo
    mock_revision = MagicMock()
    mock_revision.commit_hash = "abc123"
    mock_repo.revisions = [mock_revision]
    
    mock_cache_info = MagicMock()
    mock_cache_info.repos = [mock_repo]
    
    with patch('api.models.manager.scan_cache_dir', return_value=mock_cache_info):
        # Without specific commit
        model_no_commit = Model(hf_repo=sample_model.hf_repo, hf_commit=None)
        assert manager._has_partial_files(model_no_commit) is True
        
        # With matching commit
        model_with_commit = Model(hf_repo=sample_model.hf_repo, hf_commit="abc123")
        assert manager._has_partial_files(model_with_commit) is True
        
        # With non-matching commit
        model_wrong_commit = Model(hf_repo=sample_model.hf_repo, hf_commit="xyz789")
        assert manager._has_partial_files(model_wrong_commit) is False


@pytest.mark.asyncio
async def test_cancel_download(manager, sample_model):
    """Test cancelling a download."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_download_model') as mock_download:
        
        # Start download
        task_id = await manager.add_model(sample_model)
        
        # Cancel it
        await manager.cancel_download(sample_model)
        
        task = manager._download_tasks[task_id]
        assert task.cancelled is True


@pytest.mark.asyncio
async def test_cancel_download_not_found(manager, sample_model):
    """Test cancelling non-existent download."""
    with pytest.raises(ValueError, match="No download task found"):
        await manager.cancel_download(sample_model)


@pytest.mark.asyncio
@patch('api.models.manager.scan_cache_dir')
@patch('api.models.manager.snapshot_download')
async def test_delete_model_from_cache(mock_snapshot, mock_scan, manager, sample_model):
    """Test deleting a model from cache."""
    # Mock is_model_exist to return True (model exists in cache)
    mock_snapshot.return_value = "/tmp/test_cache"
    
    # Mock cache with the model for deletion
    revision = MockRevision("abc123")
    repo = MockRepo("test/model", [revision])
    cache_info = MockCacheInfo([repo])
    mock_scan.return_value = cache_info
    
    result = await manager.delete_model(sample_model)
    
    assert result == "deleted"


@pytest.mark.asyncio
async def test_delete_model_cancel_download(manager, sample_model):
    """Test deleting a model that's downloading cancels it."""
    with patch.object(manager, 'is_model_exist', return_value=False), \
         patch.object(manager, '_download_model', return_value=None), \
         patch.object(manager, 'cancel_download') as mock_cancel:
        
        # Start download
        await manager.add_model(sample_model)
        
        # Delete/cancel it
        result = await manager.delete_model(sample_model)
        
        assert result == "cancelled"
        mock_cancel.assert_called_once()


@patch('api.models.manager.scan_cache_dir')
def test_list_models(mock_scan, manager):
    """Test listing models with status."""
    # Mock cache with models
    revision1 = MockRevision("abc123")
    revision2 = MockRevision("def456")
    repo1 = MockRepo("test/model1", [revision1])
    repo2 = MockRepo("test/model2", [revision2])
    mock_scan.return_value = MockCacheInfo([repo1, repo2])
    
    # Mock is_model_exist to return True for first model, False for second
    def mock_exists(model):
        return model.hf_repo == "test/model1"
    
    with patch.object(manager, 'is_model_exist', side_effect=mock_exists):
        models = manager.list_models()
    
    assert len(models) == 2
    
    # Check model 1 - should be DOWNLOADED
    model1 = next(m for m in models if m.model.hf_repo == "test/model1")
    assert model1.status == ModelStatus.DOWNLOADED
    
    # Check model 2 - should be PARTIAL
    model2 = next(m for m in models if m.model.hf_repo == "test/model2")
    assert model2.status == ModelStatus.PARTIAL


@patch('api.models.manager.scan_cache_dir')
@patch('api.models.manager.shutil.disk_usage')
def test_get_disk_space(mock_disk_usage, mock_scan, manager):
    """Test getting disk space info."""
    mock_scan.return_value = MockCacheInfo([])
    
    mock_stat = Mock()
    mock_stat.free = 500000000000
    mock_disk_usage.return_value = mock_stat
    
    info = manager.get_disk_space()
    
    # 1000000 bytes = ~0.0 GB (rounds to 0.0)
    assert info.cache_size_gb == 0.0
    # 500000000000 bytes = ~465.66 GB
    assert info.available_gb == 465.66
    assert info.cache_path == manager.cache_dir


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_model_with_retry_success(mock_snapshot, manager, sample_model):
    """Test successful download with retry logic."""
    # snapshot_download is called twice:
    # 1. During download (without local_files_only)
    # 2. During verification (with local_files_only=True)
    mock_snapshot.return_value = "/tmp/test_cache"
    
    task_obj = DownloadTask(sample_model)
    await manager._download_model("test/model:abc123", sample_model, task_obj)
    
    assert task_obj.status == ModelStatus.DOWNLOADED
    assert task_obj.error_message is None
    # Should be called twice: once for download, once for verification
    assert mock_snapshot.call_count == 2


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_model_with_retry_network_error(mock_snapshot, manager, sample_model):
    """Test download with network error and retries."""
    # Simulate network error that should trigger retries
    mock_snapshot.side_effect = requests.exceptions.ConnectionError("Network error")
    
    task_obj = DownloadTask(sample_model)
    await manager._download_model("test/model:abc123", sample_model, task_obj)
    
    assert task_obj.status == ModelStatus.PARTIAL
    assert "retry attempts" in task_obj.error_message.lower()
    # Should retry 5 times (no verification call since download never succeeds)
    assert mock_snapshot.call_count == 5


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_model_with_retry_eventual_success(mock_snapshot, manager, sample_model):
    """Test download succeeds after initial failures."""
    # First 2 calls fail, 3rd succeeds (download), 4th succeeds (verification)
    mock_snapshot.side_effect = [
        requests.exceptions.ConnectionError("Network error"),
        requests.exceptions.Timeout("Timeout"),
        "/tmp/test_cache",  # Success on 3rd attempt (download)
        "/tmp/test_cache",  # Success on 4th attempt (verification)
    ]
    
    task_obj = DownloadTask(sample_model)
    await manager._download_model("test/model:abc123", sample_model, task_obj)
    
    assert task_obj.status == ModelStatus.DOWNLOADED
    assert task_obj.error_message is None
    # 2 failed download attempts + 1 successful download + 1 verification = 4 calls
    assert mock_snapshot.call_count == 4


@pytest.mark.asyncio
@patch('api.models.manager.snapshot_download')
async def test_download_verification_fails(mock_snapshot, manager, sample_model):
    """Test download with verification failure."""
    # Download succeeds but verification fails (corrupted files)
    mock_snapshot.side_effect = [
        "/tmp/test_cache",  # Download succeeds
        Exception("Checksum validation failed"),  # Verification fails
    ]
    
    task_obj = DownloadTask(sample_model)
    await manager._download_model("test/model:abc123", sample_model, task_obj)
    
    assert task_obj.status == ModelStatus.PARTIAL
    assert "verification failed" in task_obj.error_message.lower()


@patch('api.models.manager.snapshot_download')
def test_is_model_exist_verifies_files(mock_snapshot, manager, sample_model):
    """Test that is_model_exist verifies files are present."""
    # Model exists but has no files or is corrupted
    mock_snapshot.side_effect = Exception("Checksum validation failed")
    
    assert manager.is_model_exist(sample_model) is False


@patch('api.models.manager.snapshot_download')
def test_is_model_exist_with_files(mock_snapshot, manager, sample_model):
    """Test that is_model_exist succeeds when files present."""
    # Model exists with files and valid checksums
    mock_snapshot.return_value = "/tmp/test_cache/models/test/model"
    
    assert manager.is_model_exist(sample_model) is True


def test_verify_download_success(manager, sample_model):
    """Test download verification."""
    with patch.object(manager, 'is_model_exist', return_value=True):
        assert manager._verify_download_success(sample_model) is True
    
    with patch.object(manager, 'is_model_exist', return_value=False):
        assert manager._verify_download_success(sample_model) is False

