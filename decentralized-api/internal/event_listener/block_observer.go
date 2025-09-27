package event_listener

import (
	"decentralized-api/apiconfig"
	"decentralized-api/internal/event_listener/chainevents"
	"decentralized-api/logging"
	"strconv"

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
	notify                   chan struct{}
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
		notify:        make(chan struct{}, 1),
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

// UpdateStatus sets both height and caughtUp atomically and signals processing only if changed
func (bo *BlockObserver) updateStatus(newHeight int64, caughtUp bool) {
	prevHeight := bo.currentBlockHeight.Load()
	prevCaught := bo.caughtUp.Load()
	changed := (newHeight != prevHeight) || (caughtUp != prevCaught)
	if !changed {
		return
	}
	bo.currentBlockHeight.Store(newHeight)
	bo.caughtUp.Store(caughtUp)
	select {
	case bo.notify <- struct{}{}:
	default:
		// already notified; coalesce
	}
}

func (bo *BlockObserver) Process(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-bo.notify:
			// Drain extra signals to coalesce bursts
		drain:
			for {
				select {
				case <-bo.notify:
					continue
				default:
					break drain
				}
			}
			if !bo.caughtUp.Load() {
				continue
			}
			// Process as many contiguous blocks as available
			for {
				nextHeight := bo.lastProcessedBlockHeight.Load() + 1
				if nextHeight > bo.currentBlockHeight.Load() || nextHeight <= 0 {
					break
				}
				if bo.processBlock(ctx, nextHeight) {
					bo.lastProcessedBlockHeight.Store(nextHeight)
					if err := bo.ConfigManager.SetLastProcessedHeight(nextHeight); err != nil {
						logging.Warn("Failed to persist last processed height", types.Config, "error", err)
					}
				} else {
					// stop on fetch error; next status change will retry
					break
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
