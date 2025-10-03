package event_listener

import (
	"context"
	"decentralized-api/internal/utils"
	"encoding/json"
	"fmt"
	externalutils "github.com/gonka-ai/gonka-utils/go/utils"
	"strconv"
	"strings"
	"time"

	"decentralized-api/apiconfig"
	"decentralized-api/broker"
	"decentralized-api/chainphase"
	"decentralized-api/cosmosclient"
	"decentralized-api/internal/event_listener/chainevents"
	"decentralized-api/internal/poc"
	"decentralized-api/internal/validation"
	"decentralized-api/logging"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/productscience/inference/x/inference/types"
	"google.golang.org/grpc"
)

// Minimal interface for query operations needed by the dispatcher
type ChainStateClient interface {
	EpochInfo(ctx context.Context, req *types.QueryEpochInfoRequest, opts ...grpc.CallOption) (*types.QueryEpochInfoResponse, error)
	GetCurrentEpoch(ctx context.Context, req *types.QueryGetCurrentEpochRequest, opts ...grpc.CallOption) (*types.QueryGetCurrentEpochResponse, error)
	Params(ctx context.Context, req *types.QueryParamsRequest, opts ...grpc.CallOption) (*types.QueryParamsResponse, error)
}

// StatusFunc defines the function signature for getting node sync status
type StatusFunc func() (*coretypes.ResultStatus, error)

type SetHeightFunc func(blockHeight int64) error

// PoCParams contains Proof of Compute parameters
type PoCParams struct {
	StartBlockHeight int64
	StartBlockHash   string
}

// MlNodeStageReconciliationConfig defines when reconciliation should be triggered
type MlNodeStageReconciliationConfig struct {
	BlockInterval int           // Trigger every N blocks
	TimeInterval  time.Duration // OR every N time duration
}

type MlNodeReconciliationConfig struct {
	Inference       *MlNodeStageReconciliationConfig
	PoC             *MlNodeStageReconciliationConfig
	LastBlockHeight int64     // Track last reconciliation block
	LastTime        time.Time // Track last reconciliation time
}

// OnNewBlockDispatcher orchestrates processing of new block events
type OnNewBlockDispatcher struct {
	nodeBroker           *broker.Broker
	lastVerifiedAppHashHex  string
	nodePocOrchestrator  poc.NodePoCOrchestrator
	queryClient          ChainStateClient
	phaseTracker         *chainphase.ChainPhaseTracker
	reconciliationConfig MlNodeReconciliationConfig
	getStatusFunc        StatusFunc
	setHeightFunc        SetHeightFunc
	randomSeedManager    poc.RandomSeedManager
	configManager        *apiconfig.ConfigManager
	validator            *validation.InferenceValidator
	transactionRecorder     cosmosclient.InferenceCosmosClient
	isGenesisBlockProcessed bool
}

// StatusResponse matches the structure expected by getStatus function
type StatusResponse struct {
	SyncInfo SyncInfo `json:"sync_info"`
}

type SyncInfo struct {
	CatchingUp bool `json:"catching_up"`
}

var DefaultReconciliationConfig = MlNodeReconciliationConfig{
	Inference: &MlNodeStageReconciliationConfig{
		BlockInterval: 5,
		TimeInterval:  30 * time.Second,
	},
	PoC: &MlNodeStageReconciliationConfig{
		BlockInterval: 1,
		TimeInterval:  30 * time.Second,
	},
	LastTime:        time.Now(),
	LastBlockHeight: 0,
}

// NewOnNewBlockDispatcher creates a new dispatcher with default configuration
func NewOnNewBlockDispatcher(
	nodeBroker *broker.Broker,
	nodePocOrchestrator poc.NodePoCOrchestrator,
	queryClient ChainStateClient,
	phaseTracker *chainphase.ChainPhaseTracker,
	getStatusFunc StatusFunc,
	setHeightFunc SetHeightFunc,
	randomSeedManager poc.RandomSeedManager,
	reconciliationConfig MlNodeReconciliationConfig,
	configManager *apiconfig.ConfigManager,
	validator *validation.InferenceValidator,
	transactionRecorder cosmosclient.InferenceCosmosClient,
) *OnNewBlockDispatcher {
	return &OnNewBlockDispatcher{
		nodeBroker:             nodeBroker,
		nodePocOrchestrator:    nodePocOrchestrator,
		queryClient:            queryClient,
		lastVerifiedAppHashHex: configManager.GetGenesisAppHash(),
		phaseTracker:           phaseTracker,
		reconciliationConfig:   reconciliationConfig,
		getStatusFunc:          getStatusFunc,
		setHeightFunc:          setHeightFunc,
		randomSeedManager:      randomSeedManager,
		configManager:          configManager,
		transactionRecorder:    transactionRecorder,
		validator:            validator,
	}
}

// NewOnNewBlockDispatcherFromCosmosClient creates a dispatcher using a full cosmos client
// This is a convenience constructor for existing code
func NewOnNewBlockDispatcherFromCosmosClient(
	nodeBroker *broker.Broker,
	configManager *apiconfig.ConfigManager,
	nodePocOrchestrator poc.NodePoCOrchestrator,
	cosmosClient cosmosclient.CosmosMessageClient,
	phaseTracker *chainphase.ChainPhaseTracker,
	reconciliationConfig MlNodeReconciliationConfig,
	validator *validation.InferenceValidator,
	transactionRecorder cosmosclient.InferenceCosmosClient,
) *OnNewBlockDispatcher {
	// Adapt the cosmos client to our minimal interfaces
	queryClient := cosmosClient.NewInferenceQueryClient()
	setHeightFunc := func(blockHeight int64) error {
		return configManager.SetHeight(blockHeight)
	}
	getStatusFunc := func() (*coretypes.ResultStatus, error) {
		url := configManager.GetChainNodeConfig().Url
		return getStatus(url)
	}

	randomSeedManager := poc.NewRandomSeedManager(cosmosClient, configManager)

	return NewOnNewBlockDispatcher(
		nodeBroker,
		nodePocOrchestrator,
		queryClient,
		phaseTracker,
		getStatusFunc,
		setHeightFunc,
		randomSeedManager,
		reconciliationConfig,
		configManager,
		validator,
		transactionRecorder,
	)
}

// ProcessNewBlock is the main entry point for processing new block events
func (d *OnNewBlockDispatcher) ProcessNewBlock(ctx context.Context, blockInfo chainevents.FinalizedBlock, knownHeight int64) error {
	if err := d.collectGenesisBlockProof(); err != nil {
		logging.Error("failed to collect genesis block proof", types.Stages, "height", blockInfo.Block.Header.Height, "err", err)
		return err
	}

	height, err := strconv.ParseInt(blockInfo.Block.Header.Height, 10, 64)
	if err != nil {
		logging.Error("failed to parse block height", types.Stages, "height", blockInfo.Block.Header.Height, "err", err)
		return err
	}

	logging.Debug("Processing new block", types.Stages,
		"height", blockInfo.Block.Header.Height,
		"hash", blockInfo.BlockId.Hash)

	// 1. Query network for current state (sync status, epoch params)
	networkInfo, err := d.queryNetworkInfo(ctx)
	if err != nil {
		logging.Error("Failed to query network info, skipping block processing", types.Stages,
			"error", err, "height", blockInfo.Block.Header.Height)
		return err // Skip processing this block
	}

	// Update config manager height - unblock event processing
	err = d.setHeightFunc(height)
	if err != nil {
		logging.Error("Failed to write config", types.Config, "error", err)
	}

	// Fetch validation parameters - skip in tests
	if d.configManager != nil && !strings.HasPrefix(blockInfo.BlockId.Hash.String(), "686173682D") { // Skip in tests where hash has format "hash-N encoded to HEX (upper case)"
		params, err := d.queryClient.Params(ctx, &types.QueryParamsRequest{})
		if err != nil {
			logging.Error("Failed to get params", types.Validation, "error", err)
		} else {
			// Update validation parameters in config
			validationParams := apiconfig.ValidationParamsCache{
				TimestampExpiration: params.Params.ValidationParams.TimestampExpiration,
				TimestampAdvance:    params.Params.ValidationParams.TimestampAdvance,
				ExpirationBlocks:    params.Params.ValidationParams.ExpirationBlocks,
			}

			logging.Debug("Updating validation parameters", types.Validation,
				"timestampExpiration", validationParams.TimestampExpiration,
				"timestampAdvance", validationParams.TimestampAdvance,
				"expirationBlocks", validationParams.ExpirationBlocks)

			err = d.configManager.SetValidationParams(validationParams)
			if err != nil {
				logging.Warn("Failed to update validation parameters", types.Config, "error", err)
			}

			if params.Params.BandwidthLimitsParams != nil {
				bandwidthParams := apiconfig.BandwidthParamsCache{
					EstimatedLimitsPerBlockKb: params.Params.BandwidthLimitsParams.EstimatedLimitsPerBlockKb,
					KbPerInputToken:           params.Params.BandwidthLimitsParams.KbPerInputToken.ToFloat(),
					KbPerOutputToken:          params.Params.BandwidthLimitsParams.KbPerOutputToken.ToFloat(),
				}

				logging.Debug("Updated bandwidth parameters from chain", types.Config,
					"estimatedLimitsPerBlockKb", bandwidthParams.EstimatedLimitsPerBlockKb,
					"kbPerInputToken", bandwidthParams.KbPerInputToken,
					"kbPerOutputToken", bandwidthParams.KbPerOutputToken)

				err = d.configManager.SetBandwidthParams(bandwidthParams)
				if err != nil {
					logging.Warn("Failed to update bandwidth parameters", types.Config, "error", err)
				}
			}
		}
	}

	d.verifyParticipantsChain(ctx, networkInfo.BlockHeight, knownHeight)

	d.collectBlockProofs(blockInfo)
	// Let's check in prod how often this happens
	if networkInfo.BlockHeight != height {
		logging.Warn("Block height mismatch between event and network query", types.Stages,
			"event_height", height,
			"network_height", networkInfo.BlockHeight)
	}

	// 2. Update phase tracker and get phase info
	// FIXME: It looks like a problem that queries are separate inside networkInfo, and blockInfo
	// 	comes from a totally different source?
	// TODO: log block that came from event vs block returned by query
	// TODO: can we add the state to the block event? As a future optimization?

	d.phaseTracker.Update(chainphase.BlockInfo{Height: height, Hash: blockInfo.BlockId.Hash.String()}, &networkInfo.LatestEpoch, &networkInfo.EpochParams, networkInfo.IsSynced)
	epochState := d.phaseTracker.GetCurrentEpochState()
	if epochState == nil {
		logging.Error("[ILLEGAL_STATE]: Epoch state is nil right after an update call to phase tracker. "+
			"Skip block processing", types.Stages,
			"blockHeight", height, "isSynced", networkInfo.IsSynced)
		return nil
	}

	logging.Info("[new-block-dispatcher] Current epoch state.", types.Stages,
		"blockHeight", epochState.CurrentBlock.Height,
		"epoch", epochState.LatestEpoch.EpochIndex,
		"epoch.PocStartBlockHeight", epochState.LatestEpoch.PocStartBlockHeight,
		"currentPhase", epochState.CurrentPhase,
		"isSynced", epochState.IsSynced,
		"blockHash", epochState.CurrentBlock.Hash)
	logging.Debug("[new-block-dispatcher]", types.Stages, "blockHeight", epochState.CurrentBlock.Height, "blochHash", epochState.CurrentBlock.Hash)
	if !epochState.IsSynced {
		logging.Info("The blockchain node is still catching up, skipping on new block phase transitions", types.Stages)
		return nil
	}

	// 3. Check for phase transitions and stage events
	d.handlePhaseTransitions(*epochState, blockInfo)

	// 4. Check if reconciliation should be triggered
	if d.shouldTriggerReconciliation(*epochState) {
		d.triggerReconciliation(*epochState)
	}

	return nil
}

// verifyParticipantsChain verifies participants from current epoch to genesis epoch (or to latest known verified epoch) backwards
// using proof stored on-chain
// runs if verification is on AND if blocks coming with gaps (1.. 4... 8.. etc)
func (d *OnNewBlockDispatcher) verifyParticipantsChain(ctx context.Context, curHeight int64, knownHeight int64) {
	logging.Debug("verify participants", types.ParticipantsVerification, "known_height", knownHeight, "current_height", curHeight, "last_verified_app_hash", d.lastVerifiedAppHashHex)

	if d.lastVerifiedAppHashHex != "" && knownHeight != curHeight-1 {
		logging.Info("verify participants: start", types.ParticipantsVerification, "lastVerifiedAppHashHex", d.lastVerifiedAppHashHex)

		rpcClient, err := cosmosclient.NewRpcClient(d.configManager.GetChainNodeConfig().Url)
		if err != nil {
			logging.Error("Failed to create rpc client", types.ParticipantsVerification, "error", err)
			return
		}

		currEpoch, err := d.queryClient.GetCurrentEpoch(ctx, &types.QueryGetCurrentEpochRequest{})
		if err != nil {
			logging.Error("Failed to get current epoch", types.ParticipantsVerification, "error", err)
			return
		}

		logging.Info("Current epoch resolved.", types.ParticipantsVerification, "epoch", currEpoch.Epoch)

		data, err := utils.QueryActiveParticipants(rpcClient, d.transactionRecorder.NewInferenceQueryClient())(ctx, "current")
		if err != nil {
			logging.Error("Failed to get current participants data", types.ParticipantsVerification, "error", err)
			return
		}

		err = externalutils.VerifyParticipants(ctx, d.lastVerifiedAppHashHex, utils.QueryActiveParticipants(rpcClient, d.transactionRecorder.NewInferenceQueryClient()), "current")
		if err != nil {
			panic(err)
		}

		d.lastVerifiedAppHashHex = data.BlockProof.AppHashHex
		logging.Info("verify participants successfully", types.Stages, "new_app_hash", d.lastVerifiedAppHashHex)
	}
}

// Creates proof for genesis block. Is called only ones and only on genesis node
func (el *OnNewBlockDispatcher) collectGenesisBlockProof() error {
	if !el.configManager.GetChainNodeConfig().IsGenesis || el.isGenesisBlockProcessed {
		return nil
	}
	rpcClient, err := cosmosclient.NewRpcClient(el.configManager.GetChainNodeConfig().Url)
	if err != nil {
		logging.Error("Failed to create rpc client", types.ParticipantsVerification, "error", err)
		return err
	}

	// genesis block data is in block header with height=2
	genesisBlockResultsHeight := int64(2)
	block, err := rpcClient.Block(context.Background(), &genesisBlockResultsHeight)
	if err != nil {
		logging.Error("Failed to get genesis block", types.ParticipantsVerification, "error", err)
	}

	proof, err := fillValidatorsProof(chainevents.LastCommit{
		BlockId:    block.Block.LastCommit.BlockID,
		Height:     fmt.Sprintf("%v", block.Block.LastCommit.Height),
		Round:      int(block.Block.LastCommit.Round),
		Signatures: block.Block.LastCommit.Signatures,
	})
	if err != nil {
		return err
	}

	// we do not collect participants merkle proof here, because cosmos can't get merkle proof for block_height <= 1
	if err := el.transactionRecorder.SubmitActiveParticipantsPendingProof(
		&types.MsgSubmitParticipantsProof{BlockHeight: uint64(1), ValidatorsProof: proof}); err != nil {
		logging.Error("Failed to set validators proof", types.ParticipantsVerification, "error", err)
	}
	el.isGenesisBlockProcessed = true
	return nil
}

// NetworkInfo contains information queried from the network
type NetworkInfo struct {
	EpochParams types.EpochParams
	IsSynced    bool
	LatestEpoch types.Epoch
	BlockHeight int64
}

// queryNetworkInfo queries the network for sync status and epoch parameters
func (d *OnNewBlockDispatcher) queryNetworkInfo(ctx context.Context) (NetworkInfo, error) {
	// Query sync status
	status, err := d.getStatusFunc()
	if err != nil {
		return NetworkInfo{}, err
	}
	isSynced := !status.SyncInfo.CatchingUp

	epochInfo, err := d.queryClient.EpochInfo(ctx, &types.QueryEpochInfoRequest{})
	if err != nil || epochInfo == nil {
		logging.Error("Failed to query epoch info", types.Stages, "error", err)
		return NetworkInfo{}, err
	}

	return NetworkInfo{
		EpochParams: *epochInfo.Params.EpochParams,
		IsSynced:    isSynced,
		LatestEpoch: epochInfo.LatestEpoch,
		BlockHeight: epochInfo.BlockHeight,
	}, nil
}

// handlePhaseTransitions checks for and handles phase transitions and stage events
func (d *OnNewBlockDispatcher) handlePhaseTransitions(epochState chainphase.EpochState, finalizedBlock chainevents.FinalizedBlock) {
	epochContext := epochState.LatestEpoch
	blockHeight := epochState.CurrentBlock.Height
	blockHash := epochState.CurrentBlock.Hash

	// Sync broker node state with the latest epoch data at the start of a transition check
	if err := d.nodeBroker.UpdateNodeWithEpochData(&epochState); err != nil {
		logging.Error("Failed to update node with epoch data, skipping phase transitions.", types.Stages, "error", err)
		return
	}

	// Check for PoC start for the next epoch. This is the most important transition.
	if epochContext.IsStartOfPocStage(blockHeight) {

		logging.Info("DapiStage:IsStartOfPocStage: sending StartPoCEvent to the PoC orchestrator", types.Stages, "blockHeight", blockHeight, "blockHash", blockHash)
		d.randomSeedManager.GenerateSeed(epochContext.EpochIndex)
		return
	}

	// Check for PoC validation stage transitions
	if epochContext.IsEndOfPoCStage(blockHeight) {
		logging.Info("DapiStage:IsEndOfPoCStage. Calling MoveToValidationStage", types.Stages,
			"blockHeigh", blockHeight, "blockHash", blockHash)
		command := broker.NewInitValidateCommand()
		err := d.nodeBroker.QueueMessage(command)
		if err != nil {
			logging.Error("Failed to send init validate command", types.PoC, "error", err)
			return
		}
	}

	if epochContext.IsStartOfPoCValidationStage(blockHeight) {
		logging.Info("DapiStage:IsStartOfPoCValidationStage", types.Stages, "blockHeight", blockHeight, "blockHash", blockHash)
		go func() {
			d.nodePocOrchestrator.ValidateReceivedBatches(blockHeight)
		}()
	}

	if epochContext.IsEndOfPoCValidationStage(blockHeight) {
		logging.Info("DapiStage:IsEndOfPoCValidationStage", types.Stages, "blockHeight", blockHeight, "blockHash", blockHash)
		command := broker.NewInferenceUpAllCommand()
		err := d.nodeBroker.QueueMessage(command)
		if err != nil {
			logging.Error("Failed to send inference up command", types.PoC, "error", err)
			return
		}
		return
	}

	// Check for other stage transitions
	if epochContext.IsSetNewValidatorsStage(blockHeight) {
		logging.Info("DapiStage:IsSetNewValidatorsStage", types.Stages, "blockHeight", blockHeight, "blockHash", blockHash)
		go func() {
			d.randomSeedManager.ChangeCurrentSeed()
		}()
		d.collectBlockProofs(finalizedBlock)
	}

	if epochContext.IsClaimMoneyStage(blockHeight) {
		logging.Info("DapiStage:IsClaimMoneyStage", types.Stages, "blockHeight", blockHeight, "blockHash", blockHash)

		// Get the previous epoch seed for validation recovery
		previousSeed := d.configManager.GetPreviousSeed()

		// Calculate previous epoch index
		expectedPreviousEpochIndex := epochContext.EpochIndex - 1

		// Verify the seed is from the correct epoch
		if previousSeed.EpochIndex != expectedPreviousEpochIndex {
			logging.Warn("Previous seed epoch mismatch for recovery", types.Validation,
				"previousSeedEpoch", previousSeed.EpochIndex,
				"expectedPreviousEpoch", expectedPreviousEpochIndex,
				"currentEpoch", epochContext.EpochIndex)
		}

		// Execute missed validation recovery BEFORE claiming rewards
		go func() {
			// First, recover any missed validations from the previous epoch
			d.executeMissedValidationRecoveryWithSeed(expectedPreviousEpochIndex, previousSeed)

			// Then, claim rewards (this ensures we've validated everything before claiming)
			d.randomSeedManager.RequestMoney()

			// Mark the seed as claimed to prevent duplicate claims
			err := d.configManager.MarkPreviousSeedClaimed()
			if err != nil {
				logging.Error("Failed to mark seed as claimed", types.Claims, "epochIndex", expectedPreviousEpochIndex, "error", err)
			}
		}()
	}
}

// collectBlockProofs creates proofs for blocks, which active_participants_set was created in
// It gets validators signatures and participants merkle proof and stores it on-chain
func (el *OnNewBlockDispatcher) collectBlockProofs(block chainevents.FinalizedBlock) {
	height, err := strconv.ParseInt(block.Block.LastCommit.Height, 10, 64)
	if err != nil {
		logging.Error("Failed to parse block height to int", types.ParticipantsVerification, "height", block.Block.LastCommit.Height, "error", err)
		return
	}

	logging.Debug("Check if proof pending", types.ParticipantsVerification, "height", height)

	pendingProofResp, err := el.transactionRecorder.NewInferenceQueryClient().IfProofPending(context.Background(), &types.QueryIsProofPendingRequest{ProofHeight: height})
	if err != nil {
		logging.Error("Failed to check if proof is pending", types.ParticipantsVerification, "height", height, "error", err)
		return
	}

	if !pendingProofResp.Pending {
		return
	}

	logging.Info("Collecting pending proof", types.ParticipantsVerification, "height", height)

	rpcClient, err := cosmosclient.NewRpcClient(el.configManager.GetChainNodeConfig().Url)
	if err != nil {
		logging.Error("Failed to create rpc client", types.ParticipantsVerification, "error", err)
		return
	}

	proofOps, err := utils.GetParticipantsMerkleProof(rpcClient, pendingProofResp.PendingProofEpochId, height)
	if err != nil {
		logging.Error("Failed to get participants merkle proof", types.ParticipantsVerification, "error", err)
	}

	proof, err := fillValidatorsProof(block.Block.LastCommit)
	if err != nil {
		return
	}
	if err := el.transactionRecorder.SubmitActiveParticipantsPendingProof(
		&types.MsgSubmitParticipantsProof{
			BlockHeight:     uint64(height),
			ValidatorsProof: proof,
			MerkleProof:     proofOps,
		}); err != nil {
		logging.Error("Failed to set validators proof", types.ParticipantsVerification, "error", err)
	}
}

// shouldTriggerReconciliation determines if reconciliation should be triggered
func (d *OnNewBlockDispatcher) shouldTriggerReconciliation(epochState chainphase.EpochState) bool {
	switch epochState.CurrentPhase {
	case types.PoCGeneratePhase, types.PoCValidatePhase:
		return shouldTriggerReconciliation(epochState.CurrentBlock.Height, &d.reconciliationConfig, d.reconciliationConfig.PoC)
	case types.InferencePhase:
		return shouldTriggerReconciliation(epochState.CurrentBlock.Height, &d.reconciliationConfig, d.reconciliationConfig.Inference)
	case types.PoCGenerateWindDownPhase, types.PoCValidateWindDownPhase:
		return false
	}
	return false
}

func shouldTriggerReconciliation(blockHeight int64, config *MlNodeReconciliationConfig, stageConfig *MlNodeStageReconciliationConfig) bool {
	// Check block interval
	blocksSinceLastReconciliation := blockHeight - config.LastBlockHeight
	if blocksSinceLastReconciliation >= int64(stageConfig.BlockInterval) {
		return true
	}

	// Check time interval
	timeSinceLastReconciliation := time.Since(config.LastTime)
	if timeSinceLastReconciliation >= stageConfig.TimeInterval {
		return true
	}

	return false
}

// triggerReconciliation starts node reconciliation with current phase info
func (d *OnNewBlockDispatcher) triggerReconciliation(epochState chainphase.EpochState) {
	cmd, response := getCommandForPhase(epochState)
	if cmd == nil || response == nil {
		logging.Info("[triggerReconciliation] No command required for phase", types.Nodes,
			"phase", epochState.CurrentPhase, "height", epochState.CurrentBlock.Height)
		return
	}

	logging.Info("[triggerReconciliation] Created command for reconciliation", types.Nodes,
		"command_type", fmt.Sprintf("%T", cmd),
		"height", epochState.CurrentBlock.Height,
		"epoch", epochState.LatestEpoch.EpochIndex,
		"phase", epochState.CurrentPhase)

	err := d.nodeBroker.QueueMessage(cmd)
	if err != nil {
		logging.Error("[triggerReconciliation] Failed to queue reconciliation command", types.Nodes, "error", err)
		return
	}

	// Update reconciliation tracking
	d.reconciliationConfig.LastBlockHeight = epochState.CurrentBlock.Height
	d.reconciliationConfig.LastTime = time.Now()

	// Wait for a response or not?
}

func getCommandForPhase(phaseInfo chainphase.EpochState) (broker.Command, *chan bool) {
	switch phaseInfo.CurrentPhase {
	case types.PoCGeneratePhase, types.PoCGenerateWindDownPhase:
		cmd := broker.NewStartPocCommand()
		return cmd, &cmd.Response
	case types.PoCValidatePhase, types.PoCValidateWindDownPhase:
		cmd := broker.NewInitValidateCommand()
		return cmd, &cmd.Response
	case types.InferencePhase:
		cmd := broker.NewInferenceUpAllCommand()
		return cmd, &cmd.Response
	}
	return nil, nil
}

// executeMissedValidationRecoveryWithSeed performs missed validation recovery for the previous epoch
// This function runs during the Set New Validators stage to recover any missed validations
// It accepts the seed as a parameter to avoid race conditions with ChangeCurrentSeed()
func (d *OnNewBlockDispatcher) executeMissedValidationRecoveryWithSeed(previousEpochIndex uint64, previousSeed apiconfig.SeedInfo) {
	if d.validator == nil {
		logging.Warn("Missed validation recovery skipped: validator not available", types.ValidationRecovery)
		return
	}

	// Check for genesis epoch
	if previousEpochIndex == 0 && previousSeed.EpochIndex == 0 {
		logging.Info("Missed validation recovery skipped: genesis epoch", types.ValidationRecovery, "previousEpochIndex", previousEpochIndex)
		return
	}

	// Check if seed is valid
	if previousSeed.Seed == 0 {
		logging.Warn("Missed validation recovery skipped: invalid seed", types.ValidationRecovery,
			"previousEpochIndex", previousEpochIndex,
			"seedEpochIndex", previousSeed.EpochIndex)
		return
	}

	// Verify seed epoch matches (this should always be true now, but good to verify)
	if previousSeed.EpochIndex != previousEpochIndex {
		logging.Warn("Missed validation recovery skipped: seed epoch mismatch", types.ValidationRecovery,
			"previousEpochIndex", previousEpochIndex,
			"seedEpochIndex", previousSeed.EpochIndex)
		return
	}

	logging.Info("Starting missed validation recovery", types.ValidationRecovery,
		"previousEpochIndex", previousEpochIndex,
		"seed", previousSeed.Seed)

	// Detect missed validations for the previous epoch
	missedInferences, err := d.validator.DetectMissedValidations(previousEpochIndex, previousSeed.Seed)
	if err != nil {
		logging.Error("Failed to detect missed validations", types.ValidationRecovery,
			"previousEpochIndex", previousEpochIndex,
			"error", err)
		return
	}

	if len(missedInferences) == 0 {
		logging.Info("No missed validations found for recovery", types.ValidationRecovery, "previousEpochIndex", previousEpochIndex)
		return
	}

	logging.Info("Found missed validations, executing recovery", types.ValidationRecovery,
		"previousEpochIndex", previousEpochIndex,
		"missedCount", len(missedInferences))

	// Execute recovery validations
	recoveredCount, err := d.validator.ExecuteRecoveryValidations(missedInferences)
	if err != nil {
		logging.Warn("Failed to execute recovery validations", types.ValidationRecovery, "error", err)
	}

	if recoveredCount > 0 {
		logging.Info("Recovered validations", types.ValidationRecovery, "recoveredCount", recoveredCount)
		d.validator.WaitForValidationsToBeRecorded()
	}

	logging.Info("Missed validation recovery completed", types.ValidationRecovery,
		"previousEpochIndex", previousEpochIndex,
		"recoveredCount", len(missedInferences))
}

// parseNewBlockInfo extracts NewBlockInfo from a JSONRPCResponse event
func parseFinalizedBlock(event *chainevents.JSONRPCResponse) (*chainevents.FinalizedBlock, error) {
	block := chainevents.FinalizedBlock{}
	d, err := json.Marshal(event.Result.Data.Value)
	if err != nil {
		logging.Error("Failed to marshal event.Result.Data.Value", types.System, "error", err)
		return nil, err
	}
	err = json.Unmarshal(d, &block)
	if err != nil {
		logging.Error("Failed to unmarshal event.Result.Data.Value to block", types.System, "error", err)
		return nil, err
	}
	return &block, nil
}
