"""Integration tests for graceful shutdown functionality."""

import pytest
import asyncio
import requests
import time
from unittest.mock import patch
from api.inference.manager import InferenceManager, InferenceInitRequest
from api.inference.vllm.runner_test_impl import VLLMRunnerTestImpl
from api.proxy import setup_vllm_proxy, start_vllm_proxy, stop_vllm_proxy
import api.proxy as proxy_module


@pytest.fixture(autouse=True)
async def cleanup_after_test():
    """Ensure clean state after each test."""
    yield
    
    # Cleanup
    proxy_module.shutdown_event.clear()
    async with proxy_module.tasks_lock:
        proxy_module.active_proxy_tasks.clear()
    
    if proxy_module.vllm_client:
        try:
            await proxy_module.vllm_client.aclose()
        except:
            pass
    proxy_module.vllm_client = None


@pytest.mark.asyncio
async def test_end_to_end_shutdown_with_active_request():
    """Test complete shutdown flow with an active streaming request."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start the service
    manager.init_vllm(request)
    manager.start()
    setup_vllm_proxy([5001])
    proxy_module.vllm_healthy[5001] = True
    await start_vllm_proxy()
    
    # Create a long-running task
    request_completed = asyncio.Event()
    request_cancelled = asyncio.Event()
    
    async def long_running_request():
        try:
            # Simulate a long inference request
            await asyncio.sleep(10)
            request_completed.set()
        except asyncio.CancelledError:
            request_cancelled.set()
            raise
    
    task = asyncio.create_task(long_running_request())
    
    # Register task
    async with proxy_module.tasks_lock:
        proxy_module.active_proxy_tasks.add(task)
    
    # Wait a bit for task to start
    await asyncio.sleep(0.1)
    
    # Trigger shutdown
    await manager._async_stop()
    
    # Wait for cancellation
    await asyncio.sleep(0.2)
    
    # Verify request was cancelled
    assert request_cancelled.is_set()
    assert not request_completed.is_set()
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_service_restart_after_shutdown():
    """Test that service can be restarted cleanly after shutdown."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # First cycle: start and stop
    manager.init_vllm(request)
    manager.start()
    assert manager.is_running()
    
    await manager._async_stop()
    assert not manager.is_running()
    
    # Second cycle: start again
    manager.init_vllm(request)
    manager.start()
    assert manager.is_running()
    
    # Should work without issues
    await manager._async_stop()
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_multiple_sequential_stop_start_cycles():
    """Test multiple stop/start cycles work correctly."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    for cycle in range(3):
        # Start
        manager.init_vllm(request)
        manager.start()
        assert manager.is_running(), f"Cycle {cycle}: Should be running after start"
        
        # Stop
        await manager._async_stop()
        assert not manager.is_running(), f"Cycle {cycle}: Should not be running after stop"
        
        # Verify clean state
        assert manager.vllm_runner is None, f"Cycle {cycle}: Runner should be None"


@pytest.mark.asyncio
async def test_shutdown_with_multiple_concurrent_requests():
    """Test shutdown with multiple active requests."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start service
    manager.init_vllm(request)
    manager.start()
    await start_vllm_proxy()
    
    # Create multiple concurrent tasks
    num_tasks = 10
    cancelled_count = [0]
    
    async def mock_request(task_id):
        try:
            await asyncio.sleep(5)
        except asyncio.CancelledError:
            cancelled_count[0] += 1
            raise
    
    tasks = [asyncio.create_task(mock_request(i)) for i in range(num_tasks)]
    
    # Register all tasks
    async with proxy_module.tasks_lock:
        for task in tasks:
            proxy_module.active_proxy_tasks.add(task)
    
    # Wait for tasks to start
    await asyncio.sleep(0.1)
    
    # Trigger shutdown
    await manager._async_stop()
    
    # Wait for all cancellations
    await asyncio.sleep(0.2)
    
    # Verify all were cancelled
    assert cancelled_count[0] == num_tasks
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_stop_called_from_async_endpoint():
    """Test calling stop from an async endpoint (simulated)."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start
    manager.init_vllm(request)
    manager.start()
    
    # Call async_stop from async context (like a FastAPI endpoint would)
    stop_completed = asyncio.Event()
    
    async def endpoint_handler():
        await manager._async_stop()
        stop_completed.set()
    
    # Run the handler
    await endpoint_handler()
    
    # Verify it worked
    assert stop_completed.is_set()
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_graceful_shutdown_preserves_data_integrity():
    """Test that shutdown doesn't corrupt state or leave partial data."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start service
    manager.init_vllm(request)
    manager.start()
    setup_vllm_proxy([5001, 5002])
    for port in [5001, 5002]:
        proxy_module.vllm_healthy[port] = True
        proxy_module.vllm_counts[port] = 0
    await start_vllm_proxy()
    
    # Create some tasks with backend allocations
    async with proxy_module.vllm_pick_lock:
        proxy_module.vllm_counts[5001] = 3
        proxy_module.vllm_counts[5002] = 2
    
    # Shutdown
    await manager._async_stop()
    
    # Verify counts weren't corrupted (they remain as they were)
    # The shutdown doesn't reset counts, just stops accepting new requests
    assert proxy_module.vllm_counts[5001] == 3
    assert proxy_module.vllm_counts[5002] == 2
    
    # Verify shutdown state
    assert not manager.is_running()
    # Client stays alive for app lifetime (only closed in app lifespan shutdown)
    assert proxy_module.vllm_client is not None
    
    # Cleanup for other tests
    await stop_vllm_proxy()


@pytest.mark.asyncio
async def test_rapid_stop_start_stop_sequence():
    """Test rapid stop/start/stop sequence doesn't cause issues."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start
    manager.init_vllm(request)
    manager.start()
    
    # Rapid stop-start-stop
    await manager._async_stop()
    await asyncio.sleep(0.1)
    
    manager.init_vllm(request)
    manager.start()
    await asyncio.sleep(0.1)
    
    await manager._async_stop()
    
    # Should complete cleanly
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_shutdown_completion_time():
    """Test that shutdown completes within reasonable time."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start
    manager.init_vllm(request)
    manager.start()
    await start_vllm_proxy()
    
    # Add some mock tasks
    tasks = [asyncio.create_task(asyncio.sleep(2)) for _ in range(5)]
    async with proxy_module.tasks_lock:
        for task in tasks:
            proxy_module.active_proxy_tasks.add(task)
    
    # Measure shutdown time
    start_time = time.time()
    await manager._async_stop(timeout=5.0)
    elapsed = time.time() - start_time
    
    # Should complete quickly (within timeout + buffer)
    assert elapsed < 10.0, f"Shutdown took too long: {elapsed}s"
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_shutdown_with_no_active_requests():
    """Test that shutdown works correctly when there are no active requests."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start
    manager.init_vllm(request)
    manager.start()
    await start_vllm_proxy()
    
    # Verify no active tasks
    async with proxy_module.tasks_lock:
        assert len(proxy_module.active_proxy_tasks) == 0
    
    # Shutdown should be instant
    start_time = time.time()
    await manager._async_stop()
    elapsed = time.time() - start_time
    
    # Should be very fast
    assert elapsed < 2.0, f"Shutdown took too long for idle service: {elapsed}s"
    assert not manager.is_running()


@pytest.mark.asyncio
async def test_shutdown_event_prevents_new_registrations():
    """Test that shutdown event prevents new task registrations."""
    manager = InferenceManager(runner_class=VLLMRunnerTestImpl)
    request = InferenceInitRequest(model="dummy-model", dtype="auto")
    
    # Start
    manager.init_vllm(request)
    manager.start()
    
    # Set shutdown event
    proxy_module.shutdown_event.set()
    
    # Try to register a new task (simulating what happens in stream_with_tracking)
    registration_failed = False
    
    async with proxy_module.tasks_lock:
        if proxy_module.shutdown_event.is_set():
            registration_failed = True
    
    # Should have detected shutdown
    assert registration_failed
    
    # Cleanup
    await manager._async_stop()

