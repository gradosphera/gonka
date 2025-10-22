# Task 3: Implementation Plan for Graceful vLLM Inference Shutdown

## Summary

**Approach**: Implement a graceful shutdown mechanism for the vLLM inference service using `asyncio` task cancellation. This will actively interrupt in-flight proxy requests, ensure proper resource cleanup, and prevent connection leaks when the service is stopped via `/api/stop`.

**Key Actions**:
1.  Introduce state management (`asyncio.Event`, `set`) in the proxy to track and control the shutdown process.
2.  Wrap each incoming proxy request in an `asyncio.Task` to allow for active cancellation with proper race condition prevention.
3.  Enhance `InferenceManager` with an `_async_stop` method to orchestrate the shutdown: signal the proxy, cancel active tasks, close the `httpx` client, and terminate vLLM processes.
4.  Implement a synchronous, blocking `_stop` method in `InferenceManager` that properly bridges to the async shutdown, ensuring compliance with the `IManager` interface while handling calls from both sync and async contexts.
5.  Add timeout protection and comprehensive error handling to ensure shutdown completes even in failure scenarios.

---

## Context

The system uses a proxy architecture where requests to `/v1/*` are forwarded to vLLM backends. When `/api/stop` is called, the `InferenceManager` terminates vLLM processes. However, the proxy layer continues handling active connections until health checks detect that the backends are down, leaving a coordination gap.

**Current Shutdown Issues**:
- Active inference requests are abruptly interrupted.
- GPU memory may not be properly cleaned up.
- Open connections and sockets can leak.
- In-progress operations are killed mid-execution.

**Key Constraint**: LLM inference can run for minutes. Passive draining with timeouts will always fail for long requests, so **shutdown must actively interrupt ongoing requests**.

---

## Solution: AsyncIO Task Cancellation

Use `asyncio`'s task cancellation to actively interrupt in-flight proxy requests during shutdown.

**How It Works**:
1.  Track all active proxy requests as `asyncio` tasks.
2.  On shutdown, set an `asyncio.Event` to signal the proxy to reject new requests.
3.  Cancel all tracked tasks, which will trigger `CancelledError` exceptions in their handlers.
4.  Implement cleanup logic within the exception handlers to release resources correctly.
5.  Close the shared `httpx` client pool.
6.  Terminate the vLLM backend processes.

---

## Implementation Plan

### Changes to `packages/api/src/api/proxy.py`

Add global state for shutdown coordination and refactor the proxy logic to manage request tasks with atomic registration and guaranteed cleanup.

```python
import asyncio
from typing import Set
import httpx
from fastapi import Request, Response
from fastapi.responses import StreamingResponse
from starlette.middleware.base import BaseHTTPMiddleware

# ... existing imports ...

logger = create_logger(__name__)

# === Add shutdown coordination state ===
shutdown_event = asyncio.Event()
active_proxy_tasks: Set[asyncio.Task] = set()
tasks_lock = asyncio.Lock()

# ... existing globals ...

class ProxyMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        path = request.url.path
        if path.startswith("/v1"):
            return await _proxy_request_to_backend(request, request.url.path)
        return await call_next(request)


async def _proxy_request_to_backend(request: Request, backend_path: str) -> Response:
    """
    Proxy requests to vLLM backends with graceful shutdown support.
    
    The key insight: Track the streaming task (the generator), not the setup task.
    The generator runs for minutes during inference; setup completes in milliseconds.
    """
    # Basic validation
    if not vllm_backend_ports or not any(vllm_healthy.values()):
        return Response(status_code=503, content=b"No vLLM backend available")
    
    # Check shutdown before acquiring resources
    if shutdown_event.is_set():
        return Response(status_code=503, content=b"Service is shutting down")
    
    # Acquire a backend
    try:
        port = await _pick_vllm_backend()
    except RuntimeError:
        return Response(status_code=503, content=b"No vLLM backend available")
    
    # Prepare request
    if not backend_path.startswith("/"):
        backend_path = "/" + backend_path
    url = f"http://{VLLM_HOST}:{port}{backend_path}"
    headers = {k: v for k, v in request.headers.items() if k.lower() != "host"}
    
    # Check client availability
    if vllm_client is None:
        await _release_vllm_backend(port)
        return Response(status_code=503, content=b"vLLM client not initialized")
    
    # Start the upstream connection
    try:
        context_manager = vllm_client.stream(
            request.method,
            url,
            params=request.query_params,
            headers=headers,
            content=request.stream(),
            timeout=httpx.Timeout(None, read=900),
        )
        upstream = await context_manager.__aenter__()
    except Exception as exc:
        logger.exception(f"Failed to connect to vLLM backend: {exc}")
        await _release_vllm_backend(port)
        return Response(status_code=502, content=b"vLLM connection failed")
    
    # Prepare response headers
    resp_headers = {
        k: v for k, v in upstream.headers.items()
        if k.lower() not in {"content-length", "transfer-encoding", "connection"}
    }
    
    # The actual long-running streaming generator
    async def stream_with_tracking():
        """
        Generator that handles the actual streaming (the long-running part).
        This is where we track the task and handle cancellation.
        """
        # Get the task that's running THIS generator (not the setup task!)
        current_task = asyncio.current_task()
        
        # Register this streaming task for shutdown tracking
        if current_task:
            async with tasks_lock:
                if shutdown_event.is_set():
                    # Shutdown started while we were setting up
                    raise asyncio.CancelledError("Shutdown in progress")
                active_proxy_tasks.add(current_task)
        
        try:
            # Stream the response (this can take minutes for LLM inference)
            async for chunk in upstream.aiter_raw():
                yield chunk
                
        except asyncio.CancelledError:
            logger.info(f"Stream cancelled for port {port} during shutdown")
            raise
            
        finally:
            # Unregister this task - streaming is done
            if current_task:
                async with tasks_lock:
                    active_proxy_tasks.discard(current_task)
            
            # Cleanup resources
            try:
                await context_manager.__aexit__(None, None, None)
            except Exception as e:
                logger.error(f"Error closing upstream connection: {e}")
            
            await _release_vllm_backend(port)
    
    return StreamingResponse(
        stream_with_tracking(),
        status_code=upstream.status_code,
        headers=resp_headers,
    )

# ... rest of proxy code ...
```

**Key Design Points**:
1. **Track the right task**: Uses `asyncio.current_task()` inside the generator to track the actual streaming task, not the setup task. This is the task that runs for minutes, not milliseconds.
2. **Simple flow**: No nested functions or complex task wrapping. The generator registers itself, streams, and unregisters in its finally block.
3. **Guaranteed cleanup**: The finally block ensures resources are released even when the stream is cancelled mid-flight.
4. **Cancellable**: When `task.cancel()` is called during shutdown, it interrupts the `async for chunk` loop immediately, even if blocked waiting for data.

### Changes to `packages/api/src/api/inference/manager.py`

Implement async shutdown orchestration and a synchronous bridge that works correctly in all calling contexts.

```python
import asyncio
from typing import Optional
import threading
# ... other imports ...

from .. import proxy as proxy_module

class InferenceManager(IManager):
    # ... existing methods ...

    async def _async_stop(self, timeout: float = 30.0):
        """
        Graceful shutdown: signal -> try cancel -> always terminate. # proposed line
        
        [UPDATES 1]: This implementation prevents a critical bug where a timeout # proposed line
        during task cancellation would skip process termination, orphaning # proposed line
        the vLLM runner. The graceful part is allowed to time out, but the # proposed line
        essential cleanup (process termination, state reset) is guaranteed # proposed line
        to run in the `finally` block. # proposed line
        
        [UPDATES 2]: Client close is also moved to the finally block to ensure # proposed line
        it's always closed even if timeout occurs during aclose(). This prevents # proposed line
        connection leaks. # proposed line
        """
        logger.info("Starting vLLM service shutdown...")
        
        # 1. Signal to reject new requests immediately. # proposed line
        proxy_module.shutdown_event.set() # proposed line
        
        try:
            # 2. Try to gracefully cancel active streams within the timeout. # proposed line
            async with asyncio.timeout(timeout):
                # Cancel active streams
                async with proxy_module.tasks_lock:
                    tasks = list(proxy_module.active_proxy_tasks)
                    proxy_module.active_proxy_tasks.clear()
                
                if tasks:
                    logger.info(f"Cancelling {len(tasks)} active stream(s)...")
                    for task in tasks:
                        task.cancel()
                    await asyncio.gather(*tasks, return_exceptions=True)
                    
        except asyncio.TimeoutError:
            logger.warning( # proposed line
                f"Graceful shutdown timed out after {timeout}s. " # proposed line
                "Forcing termination of remaining resources." # proposed line
            )
            
        finally:
            # 3. ALWAYS close client, terminate processes, and clean up state. # proposed line
            logger.info("Terminating vLLM processes and cleaning up state...") # proposed line
            
            # Close HTTP client with its own timeout to prevent hanging # proposed line
            if proxy_module.vllm_client: # proposed line
                try: # proposed line
                    await asyncio.wait_for( # proposed line
                        proxy_module.vllm_client.aclose(), # proposed line
                        timeout=5.0 # proposed line
                    ) # proposed line
                except (asyncio.TimeoutError, Exception) as e: # proposed line
                    logger.error(f"Failed to close vllm_client: {e}") # proposed line
                finally: # proposed line
                    proxy_module.vllm_client = None # proposed line
            
            # Terminate vLLM processes (blocking operation) # proposed line
            if self.vllm_runner:
                loop = asyncio.get_running_loop() # proposed line
                await loop.run_in_executor(None, self.vllm_runner.stop) # proposed line
            
            # Reset all state for a clean restart. # proposed line
            if self._startup_task and not self._startup_task.done():
                self._startup_task.cancel()

            self.vllm_runner = None
            self._exception = None
            proxy_module.shutdown_event.clear()
            
            logger.info("Shutdown complete")

    def _stop(self):
        """
        Synchronous bridge to async shutdown (required by IManager interface).
        """
        # Cancel startup if it's running
        if self._startup_task and not self._startup_task.done():
            self._startup_task.cancel()
        
        try:
            # If event loop is running, we're in async context
            loop = asyncio.get_running_loop()
            
            # Use threading.Event to wait without blocking the loop
            done = threading.Event()
            error = [None]
            
            async def run_shutdown():
                try:
                    await self._async_stop()
                except Exception as e:
                    error[0] = e
                finally:
                    done.set()
            
            asyncio.create_task(run_shutdown())
            done.wait(timeout=35.0)
            
            if error[0]:
                raise error[0]
                
        except RuntimeError:
            # No event loop - we can use asyncio.run()
            asyncio.run(self._async_stop())
```

**Key Design Points**:
1. **Simple sequence**: Signal → Cancel → Close → Terminate. Each step is one clear action.
2. **Timeout protected**: If shutdown takes too long, we give up and clean up anyway.
3. **Threading bridge**: The `_stop` method uses `threading.Event` to wait for async work without blocking the event loop.
4. **Always cleanup**: The finally block ensures state is reset even if shutdown fails.
5. **[UPDATES 1] Guaranteed Termination**: The shutdown logic is structured to ensure that `vllm_runner.stop()` is *always* called, even if the graceful cancellation of streaming tasks times out. This prevents orphaned vLLM processes. # proposed line
6. **[UPDATES 2] Guaranteed Client Close**: The `httpx` client close is moved to the `finally` block with its own 5-second timeout. This prevents connection leaks if the main shutdown timeout is exceeded or if `aclose()` hangs. The client is always set to `None` afterward, ensuring clean state. # proposed line

---

## Testing Strategy

### Unit Tests (with Mock Server)

1.  **Test Task Cancellation During Active Request**:
    - Use `aiohttp.web` to create a mock backend with a long `asyncio.sleep()` to simulate a slow inference.
    - Start a proxy request to this backend.
    - While the request is in-flight, trigger the shutdown logic (`shutdown_event.set()`, cancel tasks).
    - **Assert**: The request task is cancelled, `active_proxy_tasks` becomes empty, and the connection count (`vllm_counts`) is decremented correctly.

2.  **Test Cleanup Runs on Cancellation**:
    - Patch `api.proxy._release_vllm_backend` with a mock that records when it's called.
    - Start and cancel a proxy request task mid-flight.
    - **Assert**: The patched release function was called, confirming the cleanup logic in the `CancelledError` handler runs.

3.  **Test New Requests Rejected During Shutdown**:
    - Set `proxy.shutdown_event` inside the `tasks_lock`.
    - Make a new request to `_proxy_request_to_backend`.
    - **Assert**: The response has a `503` status code and a "shutting down" message.

4.  **Test Task Registration Race Condition Prevention**:
    - Create a scenario where shutdown is triggered while a request is being registered.
    - Use asyncio events to control timing precisely.
    - **Assert**: Either the task is registered and then cancelled, OR rejected before registration (no orphaned tasks).

5.  **Test Concurrent Shutdown Attempts**:
    - Start the inference service.
    - Call `manager.stop()` from 3 different threads/tasks simultaneously.
    - **Assert**: No race conditions, no exceptions, clean shutdown happens exactly once.

6.  **Test Shutdown During Startup**:
    - Call `manager.start_async()` to begin startup.
    - Immediately call `manager.stop()` before startup completes.
    - **Assert**: Startup task is cancelled, no zombie processes, clean state.

7.  **Test Shutdown With Client Close Error**:
    - Mock `vllm_client.aclose()` to raise an exception.
    - Trigger shutdown.
    - **Assert**: Shutdown completes anyway, client is set to None, exception is logged.

7b. **Test Shutdown With Client Close Timeout**: # proposed line
    - Mock `vllm_client.aclose()` to hang indefinitely (e.g., await asyncio.sleep(999)). # proposed line
    - Trigger shutdown. # proposed line
    - **Assert**: Client close times out after 5 seconds, shutdown still completes, client is set to None, timeout is logged. # proposed line

8.  **Test Shutdown Timeout**:
    - Mock a task that refuses to cancel (infinite loop catching CancelledError).
    - Trigger shutdown with a short timeout (e.g., 2 seconds).
    - **Assert**: Shutdown completes after timeout, cleanup still runs in finally block.

9.  **Test Resource Cleanup Verification**:
    - After shutdown, verify:
      - `active_proxy_tasks` is empty
      - `vllm_counts` all return to zero
      - `shutdown_event` is cleared
      - `vllm_client` is None

### Integration Tests

1.  **Test End-to-End Shutdown with Active Request**:
    - Using a real FastAPI test client, start a long-running streaming inference request.
    - Use explicit synchronization (asyncio.Event) instead of time.sleep() to avoid flaky timing.
    - While the request is streaming, call the `/api/v1/stop` endpoint.
    - **Assert**: The `stop` endpoint returns a `200` status code, and the client making the long request receives a connection error or 503 response.

2.  **Test Service Restart After Shutdown**:
    - Call the shutdown endpoint.
    - Verify all resources are cleaned up.
    - Call the startup endpoint to re-initialize the service.
    - **Assert**: A subsequent inference request succeeds with a `200` status code, proving the service can restart cleanly.

3.  **Test Multiple Sequential Stop/Start Cycles**:
    - Perform: Start → Stop → Start → Stop → Start
    - **Assert**: Each cycle works cleanly without resource leaks or state corruption.

4.  **Test Shutdown with Multiple Concurrent Requests**:
    - Start 10 concurrent streaming inference requests.
    - After they're all in-flight, trigger shutdown.
    - **Assert**: All requests are cancelled, all resources cleaned up, no deadlocks.

5.  **Test Stop Called from Different Contexts**:
    - Test calling `manager.stop()` from:
      - An async endpoint (routes.py)
      - The app lifespan shutdown (app.py)
      - The health watcher (watcher.py)
    - **Assert**: Works correctly in all contexts without deadlocks or exceptions.

### Manual Validation Tests

1.  **GPU Memory Cleanup**:
    - Start inference service.
    - Run some requests.
    - Shutdown.
    - Check `nvidia-smi` to verify GPU memory is released.

2.  **Network Connection Cleanup**:
    - Start inference service.
    - Run some requests.
    - Shutdown.
    - Check `netstat -an | grep :5000` and `ss -tulpn` to verify no lingering connections.

3.  **Performance Test - No Latency Overhead**:
    - Measure average request latency without shutdown mechanism.
    - Measure average request latency with shutdown mechanism.
    - **Assert**: No significant difference (<5% overhead).

4.  **Load Test - Shutdown Under Load**:
    - Initiate 100+ concurrent requests.
    - Trigger shutdown while requests are in-flight.
    - **Assert**: Shutdown completes within timeout, system remains stable.

---

## Implementation Checklist

### 1. `proxy.py`
- [ ] Add global state: `shutdown_event`, `active_proxy_tasks`, `tasks_lock`.
- [ ] Simplify `_proxy_request_to_backend`: do all setup inline (no nested functions).
- [ ] Create `stream_with_tracking()` generator that:
  - [ ] Uses `asyncio.current_task()` to get the actual streaming task.
  - [ ] Registers itself in `active_proxy_tasks` before streaming.
  - [ ] Unregisters itself in the `finally` block after streaming.
  - [ ] Cleans up resources (`context_manager`, backend port) in the `finally` block.
- [ ] Handle early shutdown check before returning `StreamingResponse`.
- [ ] Return `StreamingResponse` with the tracking generator.

### 2. `manager.py`
- [ ] Import proxy module: `from .. import proxy as proxy_module`.
- [ ] Implement `_async_stop()`:
  - [ ] Set `shutdown_event` to reject new requests.
  - [ ] Wrap task cancellation in `asyncio.timeout()` for protection. # proposed line
  - [ ] Cancel all tasks in `active_proxy_tasks`. # proposed line
  - [ ] In `finally` block: Close `vllm_client` with its own 5s timeout. # proposed line
  - [ ] In `finally` block: Stop `vllm_runner` using `run_in_executor`. # proposed line
  - [ ] In `finally` block: Always clean up state (reset flags, clear events). # proposed line
- [ ] Implement `_stop()` as a sync bridge:
  - [ ] Use `threading.Event` to wait for async work.
  - [ ] Fall back to `asyncio.run()` if no loop is running.

### 3. Testing

#### Unit Tests
- [ ] Test task cancellation during active request (with mock server).
- [ ] Test cleanup runs on cancellation (verify `_release_vllm_backend` is called).
- [ ] Test new requests rejected during shutdown.
- [ ] Test task registration race condition prevention.
- [ ] Test concurrent shutdown attempts (multiple threads calling stop simultaneously).
- [ ] Test shutdown during startup (stop called before start completes).
- [ ] Test shutdown with client close error (aclose() raises exception).
- [ ] Test shutdown with client close timeout (aclose() hangs indefinitely). # proposed line
- [ ] Test shutdown timeout (task refuses to cancel).
- [ ] Test resource cleanup verification (all counters/flags return to expected state).

#### Integration Tests
- [ ] Test end-to-end shutdown with active request (use explicit synchronization, not sleep).
- [ ] Test service restart after shutdown.
- [ ] Test multiple sequential stop/start cycles.
- [ ] Test shutdown with multiple concurrent requests.
- [ ] Test stop called from different contexts (routes, lifespan, watcher).

#### CI Integration
- [ ] Verify all tests pass in the CI pipeline.
- [ ] Ensure tests are not flaky (use events, not timing-based waits).

### 4. Post-Implementation Validation
- [ ] **Manual: GPU Memory Cleanup** - Verify `nvidia-smi` shows clean GPU state after shutdown.
- [ ] **Manual: Network Connections** - Check `netstat` and `ss` for lingering connections.
- [ ] **Performance: Latency Overhead** - Measure request latency, ensure <5% overhead.
- [ ] **Load Test: Shutdown Under Load** - 100+ concurrent requests, verify graceful shutdown.
- [ ] **Manual: Multiple Restart Cycles** - Verify service can be stopped and restarted multiple times.

### 5. Documentation
- [ ] Update API documentation to note that `/api/v1/stop` will interrupt active requests.
- [ ] Document the shutdown timeout (30 seconds) and what happens when exceeded.
- [ ] Add comments explaining the threading.Event bridge pattern in `_stop()`.
- [ ] Document all shutdown-related log messages for operational visibility.

---

## Design Rationale

### The Core Insight: Track What's Actually Long-Running

**The Problem**: When you call `_proxy_request_to_backend()`, it does some quick setup (validating, acquiring a backend, opening a connection) and then returns a `StreamingResponse`. This setup takes milliseconds. The actual streaming—reading response chunks and sending them to the client—happens LATER when FastAPI iterates the generator, and can take **minutes** for LLM inference.

**The Mistake**: If you wrap the setup in a task and track that, the task completes immediately and you have nothing to cancel during shutdown.

**The Solution**: Track the generator task (the one FastAPI creates to iterate the response) by using `asyncio.current_task()` from inside the generator itself. This is the task that's actually long-running.

### Why AsyncIO Task Cancellation?

**Alternatives Considered**:
1. **Passive draining with timeout**: Fails for long-running LLM requests.
2. **Manual cancellation flags**: Doesn't interrupt blocking I/O.
3. **Process signals**: Too coarse, no per-request cleanup.

**Why task cancellation wins**: `task.cancel()` interrupts even blocking operations like waiting for network data. The `CancelledError` propagates through the stack, allowing cleanup at each level.

### Why Threading.Event for Sync/Async Bridge?

The `_stop()` method must be synchronous (required by the `IManager` interface) but needs to run async work. Using `threading.Event` lets us wait for async completion without blocking the event loop—the GIL is released during `wait()`, so the loop keeps running.

### Why Cleanup in Generator Finally Block?

The generator's `finally` block is guaranteed to run when:
- Streaming completes normally
- The client disconnects
- The task is cancelled during shutdown

This provides one reliable cleanup path for all cases.
