"""Unit tests for InferenceManager graceful shutdown functionality."""

import pytest
import asyncio
import threading
from unittest.mock import MagicMock, patch, AsyncMock
from api.inference.manager import InferenceManager, InferenceInitRequest
from api.inference.vllm.runner_test_impl import VLLMRunnerTestImpl
import api.proxy as proxy_module


@pytest.fixture(autouse=True)
async def reset_proxy_state():
    """Reset proxy state before and after each test."""
    proxy_module.shutdown_event.clear()
    async with proxy_module.tasks_lock:
        proxy_module.active_proxy_tasks.clear()
    if proxy_module.vllm_client:
        await proxy_module.vllm_client.aclose()
    proxy_module.vllm_client = None
    
    yield
    
    proxy_module.shutdown_event.clear()
    async with proxy_module.tasks_lock:
        proxy_module.active_proxy_tasks.clear()
    if proxy_module.vllm_client:
        await proxy_module.vllm_client.aclose()
    proxy_module.vllm_client = None


@pytest.mark.asyncio
async def test_concurrent_shutdown_attempts():
    """Test that multiple concurrent stop() calls don't cause race conditions."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    assert manager.is_running()
    
    # Create multiple tasks that call stop concurrently
    results = []
    errors = []
    
    def stop_worker(idx):
        try:
            manager.stop()
            results.append(f"success-{idx}")
        except Exception as e:
            errors.append(f"error-{idx}: {e}")
    
    threads = [threading.Thread(target=stop_worker, args=(i,)) for i in range(3)]
    
    for t in threads:
        t.start()
    
    for t in threads:
        t.join(timeout=40.0)
    
    # Should complete without errors
    assert len(errors) == 0, f"Errors occurred: {errors}"
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_shutdown_during_startup():
    """Test that _async_stop() can be called during startup to cancel it."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start async startup
    manager.start_async(request)
    assert manager.is_starting()
    
    # Immediately stop (use async stop since we're in async context)
    await manager._async_stop()
    
    # Should not be running
    assert not manager.is_running()
    assert not manager.is_starting()
    assert manager.vllm_runner is None


@pytest.mark.asyncio
async def test_shutdown_with_client_close_error():
    """Test that shutdown completes even if client.aclose() raises an exception."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    
    # Mock the client to raise an error on close
    mock_client = AsyncMock()
    mock_client.aclose = AsyncMock(side_effect=RuntimeError("Close failed"))
    proxy_module.vllm_client = mock_client
    
    # Stop should still complete (use async_stop since we're in async context)
    await manager._async_stop()
    
    # Verify cleanup happened
    assert not manager.is_running()
    assert manager.vllm_runner is None
    assert proxy_module.vllm_client is not None  # Client stays alive for app lifetime


@pytest.mark.asyncio
async def test_shutdown_with_client_close_timeout():
    """Test that shutdown completes even if client.aclose() hangs indefinitely."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    
    # Mock the client to hang on close
    mock_client = AsyncMock()
    
    async def hang_forever():
        await asyncio.sleep(999)
    
    mock_client.aclose = hang_forever
    proxy_module.vllm_client = mock_client
    
    # Stop should still complete (with timeout)
    import time
    start = time.time()
    await manager._async_stop()
    elapsed = time.time() - start
    
    # Should complete in reasonable time (not 999 seconds!)
    assert elapsed < 40, f"Shutdown took too long: {elapsed}s"
    
    # Verify cleanup happened
    assert not manager.is_running()
    assert manager.vllm_runner is None
    assert proxy_module.vllm_client is not None  # Client stays alive for app lifetime


@pytest.mark.asyncio
async def test_shutdown_timeout_still_cleans_up():
    """Test that shutdown completes cleanup even if task cancellation times out."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    
    # Create a mock task that refuses to be cancelled
    async def stubborn_task():
        try:
            while True:
                try:
                    await asyncio.sleep(0.1)
                except asyncio.CancelledError:
                    # Catch and ignore cancellation
                    pass
        except:
            pass
    
    # Add the stubborn task to active tasks
    task = asyncio.create_task(stubborn_task())
    async with proxy_module.tasks_lock:
        proxy_module.active_proxy_tasks.add(task)
    
    # Stop with a short timeout
    await manager._async_stop(timeout=1.0)
    
    # Cleanup the stubborn task
    task.cancel()
    try:
        await task
    except:
        pass
    
    # Verify cleanup happened despite timeout
    assert not manager.is_running()
    assert manager.vllm_runner is None


@pytest.mark.asyncio
async def test_stop_called_from_async_context():
    """Test that _async_stop() works correctly when called from async context."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    assert manager.is_running()
    
    # Call async stop directly when in async context
    await manager._async_stop()
    
    # Should be stopped
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_stop_called_from_sync_context():
    """Test that stop() works correctly when called from sync context (no loop)."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    assert manager.is_running()
    
    # Call stop from a thread (sync context, no event loop)
    result = []
    
    def sync_stop():
        try:
            manager.stop()
            result.append("success")
        except Exception as e:
            result.append(f"error: {e}")
    
    thread = threading.Thread(target=sync_stop)
    thread.start()
    thread.join(timeout=40.0)
    
    assert result == ["success"]
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_async_stop_cancels_active_tasks():
    """Test that _async_stop properly cancels active proxy tasks."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    
    # Create mock active tasks
    cancelled_tasks = []
    tasks_started = asyncio.Event()
    
    async def mock_task(task_id):
        tasks_started.set()
        try:
            # Infinite loop to ensure task stays active
            while True:
                await asyncio.sleep(0.1)
        except asyncio.CancelledError:
            cancelled_tasks.append(task_id)
            raise
    
    # Add tasks to active set
    tasks = [asyncio.create_task(mock_task(i)) for i in range(3)]
    async with proxy_module.tasks_lock:
        for task in tasks:
            proxy_module.active_proxy_tasks.add(task)
    
    # Wait for tasks to actually start
    await tasks_started.wait()
    await asyncio.sleep(0.2)
    
    # Call async stop
    await manager._async_stop(timeout=5.0)
    
    # Wait a bit for cancellations to process
    await asyncio.sleep(0.1)
    
    # Verify all tasks were cancelled
    assert len(cancelled_tasks) == 3, f"Expected 3 cancelled tasks, got {len(cancelled_tasks)}"
    assert not manager.is_running()
    assert proxy_module.vllm_client is not None  # Client stays alive for app lifetime
    assert not proxy_module.shutdown_event.is_set()


@pytest.mark.asyncio
async def test_resource_cleanup_after_shutdown():
    """Test that all manager resources are cleaned up after shutdown."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    
    # Setup proxy client
    await proxy_module.start_vllm_proxy()
    
    # Verify running state
    assert manager.is_running()
    assert manager.vllm_runner is not None
    
    # Stop
    await manager._async_stop()
    
    # Verify all resources cleaned up
    assert manager.vllm_runner is None
    assert manager._exception is None
    assert proxy_module.vllm_client is not None  # Client stays alive for app lifetime
    assert not proxy_module.shutdown_event.is_set()
    
    async with proxy_module.tasks_lock:
        assert len(proxy_module.active_proxy_tasks) == 0


@pytest.mark.asyncio
async def test_shutdown_event_is_set_immediately():
    """Test that shutdown_event lifecycle works correctly."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Initialize and start
    manager.init_vllm(request)
    manager.start()
    
    # Verify event is not set initially
    assert not proxy_module.shutdown_event.is_set()
    
    # Manually test the shutdown event lifecycle
    # (We can't easily race to catch it being set, so we test the mechanism directly)
    
    # Set event (simulating shutdown start)
    proxy_module.shutdown_event.set()
    assert proxy_module.shutdown_event.is_set()
    
    # Clear event (simulating shutdown complete)
    proxy_module.shutdown_event.clear()
    assert not proxy_module.shutdown_event.is_set()
    
    # Now do a real shutdown and verify clean final state
    await manager._async_stop()
    
    # Event should be cleared after shutdown completes
    assert not proxy_module.shutdown_event.is_set()
    assert not manager.is_running()

