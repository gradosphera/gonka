package event_listener

import (
	"decentralized-api/apiconfig"
	"decentralized-api/internal/event_listener/chainevents"
	"decentralized-api/logging"
	"strconv"
	"time"

	"context"
	"decentralized-api/cosmosclient"

	"sync/atomic"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/productscience/inference/x/inference/types"
)

type BlockObserver struct {
	lastProcessedBlockHeight atomic.Int64
	currentBlockHeight       atomic.Int64
	ConfigManager            *apiconfig.ConfigManager
	Queue                    *UnboundedQueue[*chainevents.JSONRPCResponse]
	caughtUp                 atomic.Bool
	tmClient                 TmHTTPClient
}

// TmHTTPClient abstracts the subset of RPC methods we need
type TmHTTPClient interface {
	BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error)
}

func NewBlockObserver(manager *apiconfig.ConfigManager) *BlockObserver {
	queue := NewUnboundedQueue[*chainevents.JSONRPCResponse]()
	// Initialize Tendermint RPC client
	httpClient, err := cosmosclient.NewRpcClient(manager.GetChainNodeConfig().Url)
	if err != nil {
		logging.Error("Failed to create Tendermint RPC client for BlockObserver", types.EventProcessing, "error", err)
	}

	bo := &BlockObserver{
		ConfigManager: manager,
		Queue:         queue,
		tmClient:      httpClient,
	}

	bo.lastProcessedBlockHeight.Store(manager.GetLastProcessedHeight())
	bo.currentBlockHeight.Store(manager.GetHeight())
	bo.caughtUp.Store(false)

	// If first run and we have a current height but no last processed, start from current-1
	if bo.lastProcessedBlockHeight.Load() == 0 && bo.currentBlockHeight.Load() > 0 {
		bo.lastProcessedBlockHeight.Store(bo.currentBlockHeight.Load() - 1)
	}

	return bo
}

func (bo *BlockObserver) UpdateBlockHeight(newHeight int64) {
	// TODO: do it in a thread-safe manner
	// We expect the update called from a different goroutine from Process
	bo.currentBlockHeight.Store(newHeight)
}

func (bo *BlockObserver) CaughtUp(caughtUp bool) {
	// TODO: same, update in a thread-safe manner
	bo.caughtUp.Store(caughtUp)
}

func (bo *BlockObserver) Process(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !bo.caughtUp.Load() {
				continue
			}
			// Process next block if available
			nextHeight := bo.lastProcessedBlockHeight.Load() + 1
			if nextHeight > bo.currentBlockHeight.Load() || nextHeight <= 0 {
				continue
			}
			if bo.processBlock(ctx, nextHeight) {
				bo.lastProcessedBlockHeight.Store(nextHeight)
				if err := bo.ConfigManager.SetLastProcessedHeight(nextHeight); err != nil {
					logging.Warn("Failed to persist last processed height", types.Config, "error", err)
				}
			}
		}
	}
}

func (bo *BlockObserver) processBlock(ctx context.Context, height int64) bool {
	if bo.tmClient == nil {
		logging.Warn("BlockObserver tmClient is nil, skipping", types.EventProcessing)
		return false
	}
	res, err := bo.tmClient.BlockResults(ctx, &height)
	if err != nil || res == nil {
		logging.Warn("Failed to fetch BlockResults", types.EventProcessing, "height", height, "error", err)
		return false
	}

	// For each tx in the block, flatten events and enqueue as synthetic Tx events
	for txIdx, txRes := range res.TxsResults {
		events := make(map[string][]string)
		// Include tx.height to satisfy waitForEventHeight
		events["tx.height"] = []string{strconv.FormatInt(height, 10)}

		for _, ev := range txRes.Events {
			evType := ev.Type
			for _, attr := range ev.Attributes {
				key := evType + "." + string(attr.Key)
				val := string(attr.Value)
				events[key] = append(events[key], val)
			}
		}

		msg := &chainevents.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      "block-" + strconv.FormatInt(height, 10) + "-tx-" + strconv.Itoa(txIdx),
			Result: chainevents.Result{
				Query:  "block_monitor/Tx",
				Data:   chainevents.Data{Type: "tendermint/event/Tx", Value: map[string]interface{}{}},
				Events: events,
			},
		}
		// Enqueue for processing
		bo.Queue.In <- msg
	}
	return true
}
