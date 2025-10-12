"""Model manager for HuggingFace models."""

import asyncio
import os
import shutil
import time
from typing import Dict, Optional, List
from pathlib import Path

from huggingface_hub import (
    scan_cache_dir,
    snapshot_download,
    HfFileSystemResolvedPath,
    list_repo_files,
    hf_hub_download,
)
from huggingface_hub.utils import (
    RepositoryNotFoundError,
    RevisionNotFoundError,
    HfHubHTTPError,
    EntryNotFoundError,
)
from tenacity import (
    retry,
    stop_after_attempt,
    wait_exponential,
    retry_if_exception_type,
    before_sleep_log,
)
import requests

from api.models.types import (
    Model,
    ModelStatus,
    ModelStatusResponse,
    DownloadProgress,
    DiskSpaceInfo,
    ModelListItem,
)
from common.logger import create_logger

logger = create_logger(__name__)

# Network-related exceptions that should trigger retries
NETWORK_EXCEPTIONS = (
    requests.exceptions.ConnectionError,
    requests.exceptions.Timeout,
    requests.exceptions.RequestException,
    HfHubHTTPError,
    TimeoutError,
    ConnectionError,
    OSError,  # Can include network-related OS errors
)


class DownloadTask:
    """Represents a running download task."""
    
    def __init__(self, model: Model):
        self.model = model
        self.task: Optional[asyncio.Task] = None
        self.start_time = time.time()
        self.error_message: Optional[str] = None
        self.status = ModelStatus.DOWNLOADING
        self.cancelled = False


class ModelManager:
    """Manages HuggingFace models in cache with download tracking."""
    
    MAX_CONCURRENT_DOWNLOADS = 3
    
    def __init__(self, cache_dir: Optional[str] = None):
        """
        Args:
            cache_dir: Optional custom HuggingFace Hub cache directory. 
                       If None, uses $HF_HOME/hub or default /root/.cache/hub.
        """
        if cache_dir:
            self.cache_dir = cache_dir
        else:
            hf_home = os.environ.get("HF_HOME", "/root/.cache")
            self.cache_dir = os.path.join(hf_home, "hub")
        
        self._download_tasks: Dict[str, DownloadTask] = {}
        self._lock = asyncio.Lock()
        logger.info(f"ModelManager initialized with cache_dir: {self.cache_dir}")
    
    def _get_task_id(self, model: Model) -> str:
        return model.get_identifier()
    
    def _has_partial_files(self, model: Model) -> bool:
        """Checks if the model has any files in cache (even if incomplete).
        
        Returns True if the repo/revision exists in cache, False otherwise.
        """
        try:
            cache_info = scan_cache_dir(self.cache_dir)
            
            # Check if repo exists in cache
            repo = next((r for r in cache_info.repos if r.repo_id == model.hf_repo), None)
            if not repo:
                return False
            
            # If specific commit requested, check if that revision exists
            if model.hf_commit:
                revision = next((r for r in repo.revisions if r.commit_hash == model.hf_commit), None)
                return revision is not None
            
            # If no commit specified, any revision counts
            return len(repo.revisions) > 0
            
        except Exception as e:
            logger.debug(f"Error checking partial files for {model.hf_repo}: {e}")
            return False
    
    def is_model_exist(self, model: Model) -> bool:
        """Checks if a model exists and is fully downloaded in the cache.
        
        Verifies all files are present and validates their checksums using
        hf_hub_download with local_files_only=True.
        """
        try:
            try:
                expected_files = list(list_repo_files(
                    repo_id=model.hf_repo,
                    revision=model.hf_commit,
                    repo_type="model"
                ))
            except Exception as e:
                logger.debug(
                    f"Failed to get file list from HuggingFace for "
                    f"{model.hf_repo}@{model.hf_commit or 'main'}: {e}"
                )
                return False
            
            if not expected_files:
                logger.debug(f"No files found in remote repo {model.hf_repo}")
                return False
            
            missing_or_corrupt = []
            for filename in expected_files:
                try:
                    hf_hub_download(
                        repo_id=model.hf_repo,
                        filename=filename,
                        revision=model.hf_commit,
                        cache_dir=self.cache_dir,
                        local_files_only=True,
                    )
                except EntryNotFoundError:
                    missing_or_corrupt.append(filename)
                except Exception as e:
                    logger.debug(f"Error verifying {filename}: {e}")
                    missing_or_corrupt.append(filename)
            
            if missing_or_corrupt:
                logger.debug(
                    f"Model {model.hf_repo}@{model.hf_commit or 'main'} incomplete: "
                    f"{len(missing_or_corrupt)}/{len(expected_files)} files missing/corrupt. "
                    f"Examples: {missing_or_corrupt[:5]}"
                )
                return False
            
            logger.info(
                f"Model {model.hf_repo}@{model.hf_commit or 'main'} verified complete "
                f"with all {len(expected_files)} files present and valid"
            )
            return True
            
        except Exception as e:
            logger.debug(
                f"Model {model.hf_repo}@{model.hf_commit or 'main'} "
                f"verification failed: {e}"
            )
            return False
    
    def _download_model_with_retry(self, model: Model) -> str:
        """Downloads a model, retrying on transient network errors.
        
        Retries up to 5 times with exponential backoff. The huggingface_hub library
        handles checksum validation and resumes partial downloads automatically.
        """
        @retry(
            stop=stop_after_attempt(5),
            wait=wait_exponential(multiplier=1, min=1, max=60),
            retry=retry_if_exception_type(NETWORK_EXCEPTIONS),
            before_sleep=before_sleep_log(logger, logger.level),
            reraise=True,
        )
        def _download_with_retry():
            logger.info(
                f"Downloading {model.hf_repo} "
                f"(commit: {model.hf_commit or 'latest'})"
            )
            return snapshot_download(
                repo_id=model.hf_repo,
                revision=model.hf_commit,
                cache_dir=self.cache_dir,
                resume_download=True,
                local_files_only=False,
            )
        
        return _download_with_retry()
    
    def _verify_download_success(self, model: Model) -> bool:
        """Verifies download integrity using checksum validation."""
        if self.is_model_exist(model):
            logger.info(f"Download verification successful: {model.hf_repo}")
            return True
        else:
            logger.error(f"Download verification failed: {model.hf_repo}")
            return False
    
    async def add_model(self, model: Model) -> str:
        """Starts a model download asynchronously.
        
        Raises:
            ValueError: If download limit is exceeded or model is already downloading.
        """
        task_id = self._get_task_id(model)
        
        async with self._lock:
            if task_id in self._download_tasks:
                existing = self._download_tasks[task_id]
                if existing.status == ModelStatus.DOWNLOADING:
                    raise ValueError(f"Model {task_id} is already downloading")
            
            active_downloads = sum(
                1 for task in self._download_tasks.values()
                if task.status == ModelStatus.DOWNLOADING
            )
            if active_downloads >= self.MAX_CONCURRENT_DOWNLOADS:
                raise ValueError(
                    f"Maximum concurrent downloads ({self.MAX_CONCURRENT_DOWNLOADS}) reached"
                )
            
            if self.is_model_exist(model):
                logger.info(f"Model {task_id} already exists in cache")
                task = DownloadTask(model)
                task.status = ModelStatus.DOWNLOADED
                self._download_tasks[task_id] = task
                return task_id
            
            download_task_obj = DownloadTask(model)
            self._download_tasks[task_id] = download_task_obj
            
            download_task_obj.task = asyncio.create_task(
                self._download_model(task_id, model, download_task_obj)
            )
        
        logger.info(f"Started download for model {task_id}")
        return task_id
    
    async def _download_model(
        self, task_id: str, model: Model, task_obj: DownloadTask
    ):
        """Downloads model with retry logic, verification, and error handling."""
        try:
            logger.info(
                f"Starting download for model {model.hf_repo} "
                f"(commit: {model.hf_commit or 'latest'}) with retry logic"
            )
            
            loop = asyncio.get_event_loop()
            
            # Timeout after 24 hours to prevent infinite hangs (large models can be 500GB+)
            await asyncio.wait_for(
                loop.run_in_executor(
                    None,
                    self._download_model_with_retry,
                    model,
                ),
                timeout=86400  # 24 hours
            )
            
            logger.info(f"Download completed for {task_id}, verifying...")
            
            if self._verify_download_success(model):
                task_obj.status = ModelStatus.DOWNLOADED
                logger.info(f"Successfully downloaded and verified model {task_id}")
            else:
                task_obj.status = ModelStatus.PARTIAL
                task_obj.error_message = "Download verification failed - model files incomplete or corrupted"
                logger.error(f"Download verification failed for {task_id}")
            
        except RepositoryNotFoundError as e:
            logger.error(f"Repository not found: {model.hf_repo}")
            task_obj.status = ModelStatus.PARTIAL
            task_obj.error_message = f"Repository not found: {model.hf_repo}"
        except RevisionNotFoundError as e:
            logger.error(f"Revision not found: {model.hf_commit}")
            task_obj.status = ModelStatus.PARTIAL
            task_obj.error_message = f"Revision not found: {model.hf_commit}"
        except asyncio.TimeoutError:
            logger.error(f"Download timeout (24 hours) for {task_id}")
            task_obj.status = ModelStatus.PARTIAL
            task_obj.error_message = "Download timeout after 24 hours"
        except asyncio.CancelledError:
            logger.info(f"Download cancelled for {task_id}")
            task_obj.status = ModelStatus.PARTIAL
            task_obj.error_message = "Download cancelled"
            # HuggingFace Hub handles partial download cleanup automatically
            raise
        except NETWORK_EXCEPTIONS as e:
            logger.error(
                f"Network error downloading model {task_id} after "
                f"5 retry attempts: {e}"
            )
            task_obj.status = ModelStatus.PARTIAL
            task_obj.error_message = (
                f"Network error after 5 retry attempts: {str(e)}"
            )
        except Exception as e:
            logger.error(f"Error downloading model {task_id}: {e}", exc_info=True)
            task_obj.status = ModelStatus.PARTIAL
            task_obj.error_message = str(e)
    
    def get_model_status(self, model: Model) -> ModelStatusResponse:
        """Gets the current status of a model.
        
        Status determination:
        - DOWNLOADING: Currently downloading (has active task)
        - DOWNLOADED: Fully downloaded and verified in cache
        - PARTIAL: Some files exist in cache but model is incomplete
        - NOT_FOUND: No trace of model in cache
        """
        task_id = self._get_task_id(model)
        
        # Check if there's an active or recent download task
        if task_id in self._download_tasks:
            task = self._download_tasks[task_id]
            
            progress = None
            if task.status == ModelStatus.DOWNLOADING:
                elapsed = time.time() - task.start_time
                progress = DownloadProgress(
                    start_time=task.start_time,
                    elapsed_seconds=elapsed
                )
            
            return ModelStatusResponse(
                model=model,
                status=task.status,
                progress=progress,
                error_message=task.error_message
            )
        
        # Check cache state (no active task)
        if self.is_model_exist(model):
            return ModelStatusResponse(
                model=model,
                status=ModelStatus.DOWNLOADED
            )
        
        # Check if there are partial files in cache
        if self._has_partial_files(model):
            return ModelStatusResponse(
                model=model,
                status=ModelStatus.PARTIAL
            )
        
        # Nothing in cache
        return ModelStatusResponse(
            model=model,
            status=ModelStatus.NOT_FOUND
        )
    
    async def cancel_download(self, model: Model):
        """Cancels an ongoing download.
        
        Raises:
            ValueError: If no download is in progress for the specified model.
        """
        task_id = self._get_task_id(model)
        
        async with self._lock:
            if task_id not in self._download_tasks:
                raise ValueError(f"No download task found for {task_id}")
            
            task = self._download_tasks[task_id]
            
            if task.status != ModelStatus.DOWNLOADING:
                raise ValueError(f"Model {task_id} is not downloading (status: {task.status})")
            
            if task.task:
                task.task.cancel()
                try:
                    await task.task
                except asyncio.CancelledError:
                    pass
            
            task.cancelled = True
            logger.info(f"Cancelled download for {task_id}")
    
    async def delete_model(self, model: Model) -> str:
        """Deletes a model from the cache or cancels an ongoing download.
        
        If `model.hf_commit` is specified, only that revision is deleted. Otherwise,
        all revisions for the repository are removed.
        
        Returns:
            "cancelled" if download was in progress, "deleted" if removed from cache.
        
        Raises:
            ValueError: If the model or specific revision is not found.
        """
        task_id = self._get_task_id(model)
        
        if task_id in self._download_tasks:
            task = self._download_tasks[task_id]
            if task.status == ModelStatus.DOWNLOADING:
                await self.cancel_download(model)
                async with self._lock:
                    del self._download_tasks[task_id]
                return "cancelled"
        
        if not self.is_model_exist(model):
            raise ValueError(f"Model {task_id} not found in cache")
        
        cache_info = scan_cache_dir(self.cache_dir)
        
        repo = next((r for r in cache_info.repos if r.repo_id == model.hf_repo), None)
        if not repo:
            raise ValueError(f"Model {task_id} not found in cache")
        
        if model.hf_commit:
            # Delete a specific revision
            revision = next((r for r in repo.revisions if r.commit_hash == model.hf_commit), None)
            if not revision:
                raise ValueError(f"Revision {model.hf_commit} not found")
            revisions_to_delete = [revision.commit_hash]
        else:
            # Delete all revisions for the repo
            revisions_to_delete = [r.commit_hash for r in repo.revisions]
        
        if not revisions_to_delete:
            raise ValueError(f"No revisions found to delete for {task_id}")
        
        strategy = cache_info.delete_revisions(*revisions_to_delete)
        logger.info(
            f"Deleting {model.hf_repo} ({len(revisions_to_delete)} revision(s)): "
            f"{strategy.expected_freed_size_str}"
        )
        strategy.execute()
        
        # Remove from download tasks cache since the model is no longer available
        if task_id in self._download_tasks:
            del self._download_tasks[task_id]
            logger.debug(f"Removed {task_id} from download tasks")
        
        return "deleted"
    
    def list_models(self) -> List[ModelListItem]:
        """Lists all models in the cache (both complete and partial).
        
        Returns models with their status:
        - DOWNLOADED: Fully downloaded and verified
        - PARTIAL: Some files exist but incomplete
        """
        models = []
        
        try:
            cache_info = scan_cache_dir(self.cache_dir)
            
            for repo in cache_info.repos:
                for revision in repo.revisions:
                    model = Model(
                        hf_repo=repo.repo_id,
                        hf_commit=revision.commit_hash
                    )
                    
                    # Determine status
                    if self.is_model_exist(model):
                        status = ModelStatus.DOWNLOADED
                    else:
                        status = ModelStatus.PARTIAL
                    
                    models.append(ModelListItem(
                        model=model,
                        status=status
                    ))
            
            downloaded_count = sum(1 for m in models if m.status == ModelStatus.DOWNLOADED)
            partial_count = sum(1 for m in models if m.status == ModelStatus.PARTIAL)
            logger.info(
                f"Found {len(models)} models in cache: "
                f"{downloaded_count} complete, {partial_count} partial"
            )
            return models
            
        except Exception as e:
            logger.error(f"Error listing models: {e}", exc_info=True)
            return []
    
    def get_disk_space(self) -> DiskSpaceInfo:
        """Gets disk space information for the cache."""
        try:
            cache_info = scan_cache_dir(self.cache_dir)
            cache_size = cache_info.size_on_disk
            
            stat = shutil.disk_usage(self.cache_dir)
            
            cache_size_gb = cache_size / (1024 ** 3)
            available_gb = stat.free / (1024 ** 3)
            
            return DiskSpaceInfo(
                cache_size_gb=round(cache_size_gb, 2),
                available_gb=round(available_gb, 2),
                cache_path=self.cache_dir
            )
            
        except Exception as e:
            logger.error(f"Error getting disk space: {e}", exc_info=True)
            return DiskSpaceInfo(
                cache_size_gb=0.0,
                available_gb=0.0,
                cache_path=self.cache_dir
            )

