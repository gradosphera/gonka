package event_listener

import (
	"decentralized-api/apiconfig"
	"decentralized-api/internal/event_listener/chainevents"
)

type BlockObserver struct {
	lastProcessedBlockHeight int64
	currentBlockHeight       int64
	ConfigManager            *apiconfig.ConfigManager
	queue                    *UnboundedQueue[*chainevents.JSONRPCResponse]
	Out                      chan *chainevents.JSONRPCResponse
	caughtUp                 bool
}

func NewBlockObserver(manager *apiconfig.ConfigManager) *BlockObserver {
	queue := &UnboundedQueue[*chainevents.JSONRPCResponse]{}
	return &BlockObserver{
		lastProcessedBlockHeight: 0,
		currentBlockHeight:       0,
		ConfigManager:            manager,
		queue:                    queue,
		Out:                      queue.Out,
		caughtUp:                 false,
	}
}

func (bo *BlockObserver) UpdateBlockHeight(newHeight int64) {
	// TODO: do it in a thread-safe manner
	// We expect the update called from a different goroutine from Process
	bo.currentBlockHeight = newHeight
}

func (bo *BlockObserver) CaughtUp(caughtUp bool) {
	// TODO: same, update in a thread-safe manner
	bo.caughtUp = caughtUp
}

func (bo *BlockObserver) Process() {
	for {
		if !bo.caughtUp {
			// TODO: sleep and wait for condition to update
			// TODO: do it optimally, so we don't sleep unnecessarily
			//  by some notification mechanism
		}
		if bo.lastProcessedBlockHeight >= bo.currentBlockHeight {
			// Sleep and wait for condition to update?
			// TODO: do it optimally, so we don't sleep unnecessarily
			//  by some notification mechanism
		}

		bo.processBlock(bo.lastProcessedBlockHeight)

		bo.lastProcessedBlockHeight++
		// TODO: Persist last processed height to the config manageer
		// that will require adding a new property to it
	}
}

func (bo *BlockObserver) processBlock(height int64) {
	// Fetch all events for the block height
	// Process them and push to the queue
}
