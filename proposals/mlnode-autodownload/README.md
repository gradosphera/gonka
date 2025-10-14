# MLNode Client - GPU and Model Management

## CRITICAL: Backward Compatibility Required

**ALL features in this task MUST be compatible with both MLNodes that have these endpoints and those without.**

- These are **complementary features, NOT blockers**
- Different MLNodes may have different endpoint availability
- When endpoints are not available: log "Skipping [feature] as endpoints are not available" and continue
- Never fail or block operations due to missing endpoint support
- Always check for `ErrAPINotImplemented` and handle gracefully

## Part 0: Client for New API

GPU monitoring and model management endpoints for the MLNode API client.

### Error Handling

Methods return `ErrAPINotImplemented` when ML node doesn't support the endpoint (older versions):

```go
if errors.Is(err, &mlnodeclient.ErrAPINotImplemented{}) {
    // Handle unsupported API
}
```

### GPU Operations

#### GetGPUDevices
`GET /api/v1/gpu/devices`

Returns CUDA device information. Empty list if no GPUs or NVML unavailable.

```go
resp, err := client.GetGPUDevices(ctx)
// resp.Devices[i]: Index, Name, TotalMemoryMB, FreeMemoryMB, UsedMemoryMB,
//                  UtilizationPercent, TemperatureC, IsAvailable, ErrorMessage
```

#### GetGPUDriver
`GET /api/v1/gpu/driver`

Returns driver version info: `DriverVersion`, `CudaDriverVersion`, `NvmlVersion`

### Model Management

#### CheckModelStatus
`POST /api/v1/models/status`

Check model cache status. Returns: `DOWNLOADED`, `DOWNLOADING`, `NOT_FOUND`, or `PARTIAL`

```go
model := mlnodeclient.Model{
    HfRepo: "meta-llama/Llama-2-7b-hf",
    HfCommit: nil, // nil = latest
}
status, err := client.CheckModelStatus(ctx, model)
```

Response includes `Progress` field when `DOWNLOADING` (contains `StartTime`, `ElapsedSeconds`)

#### DownloadModel
`POST /api/v1/models/download`

Start async download. Max 3 concurrent downloads.
- Returns 409 if already downloading
- Returns 429 if limit reached

```go
resp, err := client.DownloadModel(ctx, model)
// resp.TaskId, resp.Status, resp.Model
```

#### DeleteModel
`DELETE /api/v1/models`

Delete model or cancel download:
- `HfCommit` set: deletes specific revision
- `HfCommit` nil: deletes all versions
- Returns "deleted" or "cancelled" status

#### ListModels
`GET /api/v1/models/list`

List all cached models with status.

```go
resp, err := client.ListModels(ctx)
// resp.Models[i]: Model{HfRepo, HfCommit}, Status
```

#### GetDiskSpace
`GET /api/v1/models/space`

Returns `CacheSizeGB`, `AvailableGB`, `CachePath`

### Implementation

Code organization:
- `types.go` - Response types
- `errors.go` - Error types
- `gpu.go` - GPU methods
- `models.go` - Model methods
- `interface.go` - MLNodeClient interface
- `mock.go` - Mock client with state tracking and error injection
- `client.go` - Base client
- `poc.go` - PoC methods

Mock client supports full state tracking, error injection, call counters, and parameter capture for testing.

## Part 1: Pre-download Models for Next Epoch

### Overview

Automatic model downloading for next epoch to ensure MLNodes have required models before they're needed.

### Design: ModelWeightManager

Isolated component that runs independently of broker operations. Checks periodically (every 30 minutes) if MLNodes have downloaded models for upcoming epochs.

### Architecture Decision: Isolated vs Broker-Integrated

**Chosen approach: Isolated component**

Reasons:
- Complementary feature that shouldn't block core operations
- Clear separation of concerns
- Easier to disable if endpoints unavailable
- No risk of interfering with broker's reconciliation logic
- Can be tested independently

Dependencies:
- `configManager` - Get node configurations and model lists
- `phaseTracker` - Get current epoch state and timing
- `mlNodeClientFactory` - Create clients to communicate with MLNodes

### Download Window Logic

Only attempt downloads during safe window:

**Conditions (all must be true):**
- Current phase is `Inference`
- Current block >= `SetNewValidators + 30 blocks`
- Current block <= `InferenceValidationCutoff - 200 blocks`

**Example calculation:**
```
set_new_validators: 796601
inference_validation_cutoff: 811607

Window start: 796601 + 30 = 796631
Window end: 811607 - 200 = 811407

Current block 805564 is WITHIN window ✓
```

### Implementation Structure

```go
type ModelWeightManager struct {
    configManager       ConfigManagerInterface
    phaseTracker        PhaseTrackerInterface
    mlNodeClientFactory mlnodeclient.ClientFactory
    checkInterval       time.Duration
}

func NewModelWeightManager(
    configManager ConfigManagerInterface,
    phaseTracker PhaseTrackerInterface,
    clientFactory mlnodeclient.ClientFactory,
    checkInterval time.Duration,
) *ModelWeightManager

func (m *ModelWeightManager) Start(ctx context.Context)
```

### Algorithm

Every 30 minutes:

1. **Check if in download window:**
   - Get current epoch state from `phaseTracker`
   - Verify phase is `Inference`
   - Calculate window: `[SetNewValidators + 30, InferenceValidationCutoff - 200]`
   - If outside window: skip and wait for next check

2. **Get nodes and models:**
   - Get all nodes from `configManager.GetNodes()`
   - For each node, extract configured models from local config (`node.Models` map keys)

3. **Check and trigger downloads:**
   - For each node, create MLNodeClient
   - For each model in node's local config:
     - Call `CheckModelStatus(ctx, model)`
     - If `ErrAPINotImplemented`: log INFO "Endpoint not available for node X", continue to next node
     - If status is `NOT_FOUND` or `PARTIAL`: call `DownloadModel(ctx, model)`
     - If status is `DOWNLOADED` or `DOWNLOADING`: skip (already handled)
     - Log downloads: "Pre-downloading model X for node Y"

4. **Error handling:**
   - Never fail/panic on errors
   - Log warnings for network failures
   - Continue to next node/model on errors
   - Gracefully handle `ErrAPINotImplemented` (older MLNode versions)

### Integration Point

Initialize and start in `main.go` after broker initialization:

```go
modelWeightManager := NewModelWeightManager(
    configManager,
    phaseTracker,
    &mlnodeclient.HttpClientFactory{},
    30 * time.Minute,
)
go modelWeightManager.Start(ctx)
```

### Error Scenarios

**Endpoint not available:**
```
Log: "Model pre-download endpoint not available for node ml-node-1"
Action: Continue to next node
```

**Download already in progress:**
```
Log: "Model meta-llama/Llama-2-7b-hf already downloading on node ml-node-1"
Action: Continue to next model
```

**Network error:**
```
Log: "Failed to check model status on node ml-node-1: connection refused"
Action: Continue to next node
```

### Testing Considerations

- Mock `ConfigManager` to return test nodes/models
- Mock `ChainPhaseTracker` to control epoch state/timing
- Mock `MLNodeClient` to simulate different status responses
- Test window boundary conditions
- Test graceful handling of `ErrAPINotImplemented`

### Implementation Details

**Files created:**
- `decentralized-api/internal/modelmanager/model_manager.go` - Main implementation
- `decentralized-api/internal/modelmanager/model_manager_test.go` - Comprehensive tests

**Key features:**
- Interface-based design for easy testing (`ConfigManagerInterface`, `PhaseTrackerInterface`)
- Factory pattern for client creation (consistent with Broker)
- Window validation using epoch context methods (`SetNewValidators()`, `InferenceValidationCutoff()`)
- Minimal logging for endpoint unavailability (single INFO message per node)
- Graceful error handling with `errors.As()` for `ErrAPINotImplemented`
- Continues checking all models even if one fails
- URL construction with version support for future upgrades

**Integration:**
- Added to `main.go` after broker initialization (line 143-149)
- Runs as independent goroutine with 30-minute check interval
- Uses `mlnodeclient.HttpClientFactory` (consistent with Broker pattern)
- Properly integrated with main context for graceful shutdown

**Test coverage:**
- Window boundary tests (before start, after end, inside window)
- Model status tests (not found, partial, downloading, downloaded)
- Error handling tests (endpoint not implemented, network errors)
- Multiple models configuration
- URL formatting tests

## Part 2: Fetch GPU Info and Add to Hardware Nodes

### Overview

Automatic GPU hardware detection added to MLNodeBackgroundManager (renamed from ModelWeightManager) to collect GPU information and propagate to config, broker, and chain.

### Design: Simple Extension

Simple approach - extend existing manager with second method in same ticker loop.

**Key decisions:**
- Rename `ModelWeightManager` → `MLNodeBackgroundManager`
- Single goroutine, single 30-minute ticker for both operations
- Dual update pattern - config (persistence) + broker (immediate effect)
- GPU format: `"NVIDIA RTX 3090 | 24GB"` (pipe separator, memory in GB)
- Non-blocking - gracefully handles `ErrAPINotImplemented`, network errors

### GPU Transformation Logic

Transform `GPUDevice` list to `Hardware` entries:
- Group GPUs by: `"{Name} | {MemoryGB}GB"`
- Convert MB → GB (divide by 1024)
- Count identical GPUs
- Skip unavailable (`IsAvailable: false`)
- Skip errored (`ErrorMessage != nil`)
- Skip without memory info (`TotalMemoryMB == nil`)
- Sort alphabetically for consistency

**Example:**
```
Input: [
  {Name: "NVIDIA RTX 3090", TotalMemoryMB: 24576, IsAvailable: true},
  {Name: "NVIDIA RTX 3090", TotalMemoryMB: 24576, IsAvailable: true},
  {Name: "NVIDIA RTX 4090", TotalMemoryMB: 24576, IsAvailable: true},
]

Output: [
  {Type: "NVIDIA RTX 3090 | 24GB", Count: 2},
  {Type: "NVIDIA RTX 4090 | 24GB", Count: 1},
]
```

### Architecture

**Simple structure:**
```go
func (m *MLNodeBackgroundManager) Start(ctx context.Context) {
    ticker := time.NewTicker(m.checkInterval) // 30 minutes
    for {
        select {
        case <-ticker.C:
            m.checkAndDownloadModels(ctx)  // Existing
            m.checkAndUpdateGPUs(ctx)       // New
        case <-ctx.Done():
            return
        }
    }
}
```

**Flow:**
```
checkAndUpdateGPUs()
  ├─> For each node:
  │   ├─> GET /api/v1/gpu/devices
  │   ├─> transformGPUDevicesToHardware()
  │   ├─> Update node.Hardware
  │   └─> Send UpdateNodeHardwareCommand to broker
  └─> configManager.SetNodes() (batch persist)

Broker receives UpdateNodeHardwareCommand
  └─> Updates b.nodes[id].Hardware
       └─> Next syncNodes() → Chain
```

### Implementation

#### 1. UpdateNodeHardwareCommand

**File:** `decentralized-api/broker/node_admin_commands.go`

Simple command updates Hardware field:

```go
type UpdateNodeHardwareCommand struct {
    NodeId   string
    Hardware []apiconfig.Hardware
    Response chan error
}

func (c UpdateNodeHardwareCommand) Execute(b *Broker) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    node, exists := b.nodes[c.NodeId]
    if !exists {
        c.Response <- fmt.Errorf("node not found: %s", c.NodeId)
        return
    }
    
    node.Node.Hardware = c.Hardware
    c.Response <- nil
}
```

Registered in `broker.go`:
- `executeCommand()` switch case
- Low-priority command (default queue) - not time-critical

#### 2. Extend Manager

**File:** `mlnode_background_manager.go` (renamed from `model_manager.go`)

Add broker interface and field:
```go
type BrokerInterface interface {
    QueueMessage(command broker.Command) error
}

type MLNodeBackgroundManager struct {  // Renamed
    configManager       ConfigManagerInterface
    phaseTracker        PhaseTrackerInterface
    broker              BrokerInterface  // Added
    mlNodeClientFactory mlnodeclient.ClientFactory
    checkInterval       time.Duration
}
```

Update `Start()` to call both methods:
```go
func (m *MLNodeBackgroundManager) Start(ctx context.Context) {
    ticker := time.NewTicker(m.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            m.checkAndDownloadModels(ctx)  // Existing
            m.checkAndUpdateGPUs(ctx)       // New
        case <-ctx.Done():
            return
        }
    }
}
```

Add GPU methods:
```go
func (m *MLNodeBackgroundManager) checkAndUpdateGPUs(ctx context.Context) {
    nodes := m.configManager.GetNodes()
    updatedNodes := make([]apiconfig.InferenceNodeConfig, len(nodes))
    copy(updatedNodes, nodes)
    
    for i := range updatedNodes {
        node := &updatedNodes[i]
        hardware, err := m.fetchNodeGPUHardware(ctx, node)
        
        if err != nil {
            // Handle ErrAPINotImplemented gracefully
            continue
        }
        
        if len(hardware) == 0 {
            continue
        }
        
        // Update config
        node.Hardware = hardware
        
        // Update broker
        cmd := broker.UpdateNodeHardwareCommand{
            NodeId: node.Id, Hardware: hardware, Response: make(chan error, 1),
        }
        m.broker.QueueMessage(cmd)
        <-cmd.Response
    }
    
    m.configManager.SetNodes(updatedNodes)
}

func transformGPUDevicesToHardware(devices []mlnodeclient.GPUDevice) []apiconfig.Hardware {
    groupCounts := make(map[string]uint32)
    for _, device := range devices {
        if !device.IsAvailable || device.ErrorMessage != nil || device.TotalMemoryMB == nil {
            continue
        }
        memoryGB := *device.TotalMemoryMB / 1024
        key := fmt.Sprintf("%s | %dGB", device.Name, memoryGB)
        groupCounts[key]++
    }
    
    hardware := make([]apiconfig.Hardware, 0, len(groupCounts))
    for gpuType, count := range groupCounts {
        hardware = append(hardware, apiconfig.Hardware{Type: gpuType, Count: count})
    }
    sort.Slice(hardware, func(i, j int) bool { return hardware[i].Type < hardware[j].Type })
    return hardware
}
```

#### 3. Update Tests

**File rename:** `model_manager_test.go` → `mlnode_background_manager_test.go`

Add test cases:
- GPU transformation with identical GPUs (count aggregation)
- Mixed GPU types
- Skip unavailable/errored GPUs
- ErrAPINotImplemented handling
- Network error handling
- Empty GPU list
- Broker update success/failure

#### 4. Integration

**File:** `decentralized-api/main.go`

Updated initialization:

```go
mlnodeBackgroundManager := modelmanager.NewMLNodeBackgroundManager(
    config,
    chainPhaseTracker,
    nodeBroker,  // Added broker
    &mlnodeclient.HttpClientFactory{},
    30*time.Minute,
)
go mlnodeBackgroundManager.Start(ctx)
```

### Error Handling

Graceful, non-blocking error handling:

**Endpoint not available:**
- Log INFO: "GPU endpoint not available for node X"
- Continue to next node

**Network errors:**
- Log WARN: "Failed to fetch GPU info for node X: {error}"
- Continue to next node, retry next cycle (30 min)

**Broker failures:**
- Log WARN: "Failed to update broker hardware for node X: {error}"
- Config still persisted, continues to next node

**Config save failure:**
- Log ERROR: "Failed to persist GPU hardware to config: {error}"
- Broker already updated, continues operation

### Summary

**Simple extension approach:**
- Renamed `ModelWeightManager` → `MLNodeBackgroundManager`
- Added broker parameter and GPU methods (~100 lines)
- Single ticker, both operations every 30 minutes
- Dual update: broker (immediate) + config (persistence)
- GPU format: `"GPU Name | XXG B"` with pipe separator
- Comprehensive tests with full error coverage
- All existing tests pass, no breaking changes