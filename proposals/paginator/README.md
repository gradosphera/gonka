**AI Agent Instructions:**

- **Implement this solution exactly as described.** Do not deviate from the provided code patterns.
- **Cover all changes with comprehensive tests.**
- **Run tests after each file change using `go test -count=1 ./...`** to verify that the pagination issue is resolved and no regressions are introduced.
- **Never use emojis in any output or communication.**
- **Create the pagination utility files with correct package declarations before implementing the fixes.**
- **Update all variable references after changing from single query to paginated approach.**

-----

# Pagination Fixes for `All` Queries

## Problem

In Cosmos SDK, when `PageRequest` is nil or `Limit` is 0, the default page size is 100. These calls return only the first 100 items and silently miss the rest:

1. **`inference-chain/x/inference/keeper/accountsettle.go:99`** - `SettleAccounts` misses participants beyond first 100
2. **`decentralized-api/internal/server/public/get_participants_handler.go:193`** - HTTP endpoint returns incomplete data
3. **`decentralized-api/cosmosclient/cosmosclient.go:395`** - `GetPartialUpgrades` misses upgrade plans

## Solution

Add a simple wrapper function and use appropriate strategy per case.

### Pagination Wrapper

Imports required for the helper:
- fmt
- github.com/cosmos/cosmos-sdk/types/query

```go
// Add to decentralized-api/utils/pagination.go (package utils)
// Copy to inference-chain/x/inference/keeper/pagination.go (package keeper)
func GetAllWithPagination[T any](
	queryFunc func(*query.PageRequest) ([]T, *query.PageResponse, error),
) ([]T, error) {
	var allItems []T
	var nextKey []byte

	for {
		req := &query.PageRequest{
			Key:   nextKey,
			Limit: 1000,
		}

		items, pagination, err := queryFunc(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page (items so far: %d): %w", len(allItems), err)
		}

		allItems = append(allItems, items...)

		if pagination == nil || len(pagination.NextKey) == 0 {
			break
		}
		nextKey = pagination.NextKey
	}

	return allItems, nil
}
```

**Required file creation:**
1. Create `decentralized-api/utils/pagination.go` with package declaration:
   ```go
   package utils
   
   import (
       "fmt"
       "github.com/cosmos/cosmos-sdk/types/query"
   )
   ```

2. Create `inference-chain/x/inference/keeper/pagination.go` with package declaration:
   ```go
   package keeper
   
   import (
       "fmt"
       "github.com/cosmos/cosmos-sdk/types/query"
   )
   ```

### Consistency and Block Pinning

- **On-chain settlement (critical):** Run all reads inside the same settlement block `sdk.Context` and never reach out to external RPC/gRPC from keeper logic. Key-based pagination over the KVStore is snapshot-consistent within a single block context.
- **Off-chain gRPC/HTTP callers:** Pin all paginated queries to a single height. Use the gRPC header `x-cosmos-block-height` to ensure every page is served from the same block. You can either:
  - Capture the height from the first page’s response headers and reuse it for subsequent pages, or
  - Proactively set the height (e.g., via Tendermint RPC `/status`) and attach it to the outgoing context for all pages.

## Fixes

### 1. SettleAccounts - Use Wrapper (Read All, Same Block)

**Current:**
```go
participants, err := k.ParticipantAll(ctx, &types.QueryAllParticipantRequest{})
```

**Fixed (runs entirely within the settlement block `sdk.Context`):**
Note: Ensure the helper is available in the keeper package (e.g., `x/inference/keeper/pagination.go`).
```go
allParticipants, err := GetAllWithPagination(func(pageReq *query.PageRequest) ([]types.Participant, *query.PageResponse, error) {
	resp, err := k.ParticipantAll(ctx, &types.QueryAllParticipantRequest{Pagination: pageReq})
	if err != nil {
		return nil, nil, err
	}
	return resp.Participant, resp.Pagination, nil
})
if err != nil {
	k.LogError("Error getting all participants", types.Settle, "error", err)
	return err
}

k.LogInfo("Got all participants", types.Settle, "participants", len(allParticipants))
// Continue with existing logic using allParticipants instead of participants.Participant
// Update all references from participants.Participant to allParticipants in the settlement logic
```

Notes:
- This code executes inside keeper logic using the same `ctx` provided to the settlement, guaranteeing a consistent snapshot for the entire pagination.
- Do not perform external network queries from settlement logic.

### 2. getAllParticipants - Process Per Page (Pinned Height)

**Current:**
```go
r, err := queryClient.ParticipantAll(ctx.Request().Context(), &types.QueryAllParticipantRequest{})
```

**Fixed (pin to a single block height using gRPC metadata):**
Imports needed:
- context
- fmt
- grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
- google.golang.org/grpc
- google.golang.org/grpc/metadata
- strconv
```go
var participants []ParticipantDto
var nextKey []byte
var pinnedCtx context.Context
var blockHeight int64

// First page: capture height from response headers
{
	var hdr metadata.MD
	req := &types.QueryAllParticipantRequest{
		Pagination: &query.PageRequest{Key: nil, Limit: 1000},
	}
	resp, err := queryClient.ParticipantAll(ctx.Request().Context(), req, grpc.Header(&hdr))
	if err != nil {
		return err
	}
	// Pin height for subsequent pages
	heights := hdr.Get(grpctypes.GRPCBlockHeightHeader)
	if len(heights) == 0 {
		return fmt.Errorf("missing %s header", grpctypes.GRPCBlockHeightHeader)
	}
	pinnedCtx = metadata.NewOutgoingContext(ctx.Request().Context(), metadata.Pairs(grpctypes.GRPCBlockHeightHeader, heights[0]))
    if h, err := strconv.ParseInt(heights[0], 10, 64); err == nil {
        blockHeight = h
    }

	// Convert this first page immediately
	for _, p := range resp.Participant {
		balances, err := s.recorder.BankBalances(pinnedCtx, p.Address)
		pBalance := int64(0)
		if err == nil {
			for _, balance := range balances {
				if balance.Denom == "ngonka" {
					pBalance = balance.Amount.Int64()
				}
			}
		}
		participants = append(participants, ParticipantDto{
			Id:          p.Address,
			Url:         p.InferenceUrl,
			CoinsOwed:   p.CoinBalance,
			RefundsOwed: 0, // or p.RefundsOwed if available
			Balance:     pBalance,
			VotingPower: int64(p.Weight),
			Reputation:  0, // or p.Reputation if available
		})
	}
	if resp.Pagination != nil {
		nextKey = resp.Pagination.NextKey
	}
}

for {
	req := &types.QueryAllParticipantRequest{
		Pagination: &query.PageRequest{Key: nextKey, Limit: 1000},
	}
	resp, err := queryClient.ParticipantAll(pinnedCtx, req)
	if err != nil {
		return err
	}
	
	// Convert this page immediately
	for _, p := range resp.Participant {
		balances, err := s.recorder.BankBalances(pinnedCtx, p.Address)
		pBalance := int64(0)
		if err == nil {
			for _, balance := range balances {
				if balance.Denom == "ngonka" {
					pBalance = balance.Amount.Int64()
				}
			}
		}
		participants = append(participants, ParticipantDto{
			Id:          p.Address,
			Url:         p.InferenceUrl,
			CoinsOwed:   p.CoinBalance,
			RefundsOwed: 0, // or p.RefundsOwed if available
			Balance:     pBalance,
			VotingPower: int64(p.Weight),
			Reputation:  0, // or p.Reputation if available
		})
	}
	
	if resp.Pagination == nil || len(resp.Pagination.NextKey) == 0 {
		break
	}
	nextKey = resp.Pagination.NextKey
}
// Continue with participants slice
```
When returning the DTO, include `BlockHeight: blockHeight`.

### 3. GetPartialUpgrades - Use Wrapper (Read All, Prefer Pinned Height)

**Current:**
```go
func (icc *InferenceCosmosClient) GetPartialUpgrades() (*types.QueryAllPartialUpgradeResponse, error) {
	return icc.NewInferenceQueryClient().PartialUpgradeAll(icc.ctx, &types.QueryAllPartialUpgradeRequest{})
}
```

**Fixed:**
Imports needed:
- decentralized-api/utils
- github.com/cosmos/cosmos-sdk/types/query
```go
func (icc *InferenceCosmosClient) GetPartialUpgrades() (*types.QueryAllPartialUpgradeResponse, error) {
	// Recommended: ensure icc.ctx is already pinned to a single height via metadata
	// (caller can wrap icc.ctx with metadata.Pairs(grpctypes.GRPCBlockHeightHeader, strconv.FormatInt(height, 10))).

    allUpgrades, err := utils.GetAllWithPagination(func(pageReq *query.PageRequest) ([]types.PartialUpgrade, *query.PageResponse, error) {
		resp, err := icc.NewInferenceQueryClient().PartialUpgradeAll(icc.ctx, &types.QueryAllPartialUpgradeRequest{Pagination: pageReq})
		if err != nil {
			return nil, nil, err
		}
		return resp.PartialUpgrade, resp.Pagination, nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryAllPartialUpgradeResponse{
		PartialUpgrade: allUpgrades,
		Pagination:     &query.PageResponse{Total: uint64(len(allUpgrades))},
	}, nil
}
```

## Implementation Notes

- **Utility location**: Place `GetAllWithPagination` in `decentralized-api/utils/pagination.go`; copy the same function into `inference-chain/x/inference/keeper/pagination.go` for on-chain usage.
- **Error context**: Enhanced error messages show progress when failures occur
- **Two strategies**:
  - Use wrapper for business logic needing complete datasets (SettleAccounts, GetPartialUpgrades)
  - Process per-page for memory-efficient transformations (getAllParticipants)
- **Page size 1000**: Efficient balance between API calls and memory usage
- **Pattern proven**: Based on existing successful implementation in `inference_validation.go`

### Required Test Coverage

Create comprehensive tests for each fix:

1. **SettleAccounts tests** (`inference-chain/x/inference/keeper/accountsettle_test.go`):
   - Test with >100 participants to verify pagination works
   - Test settlement consistency across pages
   - Test error handling during pagination

2. **getAllParticipants tests** (`decentralized-api/internal/server/public/get_participants_handler_test.go`):
   - Test HTTP endpoint with >100 participants
   - Verify block height pinning works correctly
   - Test DTO conversion with all fields

3. **GetPartialUpgrades tests** (`decentralized-api/cosmosclient/cosmosclient_test.go`):
   - Test with >100 partial upgrades
   - Verify wrapper function integration
   - Test response structure integrity

### Settlement Consistency (extremely important)

- All settlement logic runs inside a single block’s `sdk.Context`. Using the wrapper with the keeper’s `ctx` guarantees a consistent view of state across all pages.
- Never perform external node queries from within settlement; rely solely on keeper/store reads in the provided context.

### gRPC Height Pinning Tips

- Header key: `x-cosmos-block-height` (use `grpctypes.GRPCBlockHeightHeader`).
- To pin:
  - Capture from first response via `grpc.Header(&md)` and reuse with `metadata.NewOutgoingContext`.
  - Or prefetch a height (e.g., Tendermint RPC `/status`) and set it in the outgoing context before any page call.
  - When building HTTP DTOs, parse the header (string to int64) and store it, e.g., `ParticipantsDto.BlockHeight`.

This fixes the critical data integrity issue where only first 100 items are returned, using minimal, safe patterns already proven in the codebase.

### Implementation Checklist

Before implementing, ensure you understand:
1. **File structure**: Create utility files with correct package declarations first
2. **Variable updates**: Change `participants.Participant` to `allParticipants` in settlement logic
3. **Context usage**: Use `pinnedCtx` for all subsequent gRPC calls after height capture
4. **Error handling**: Maintain existing error patterns while adding pagination context
5. **Testing**: Run tests after each change to verify no regressions
6. **Bank queries**: Include bank balance queries in participant conversion logic
7. **DTO fields**: Populate all ParticipantDto fields including RefundsOwed and Reputation