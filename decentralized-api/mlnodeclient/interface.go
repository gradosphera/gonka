package mlnodeclient

import "context"

// MLNodeClient defines the interface for interacting with ML nodes
type MLNodeClient interface {
	// Training operations
	StartTraining(ctx context.Context, taskId uint64, participant string, nodeId string, masterNodeAddr string, rank int, worldSize int) error
	GetTrainingStatus(ctx context.Context) error

	// Node state operations
	Stop(ctx context.Context) error
	NodeState(ctx context.Context) (*StateResponse, error)

	// PoC operations
	GetPowStatus(ctx context.Context) (*PowStatusResponse, error)
	InitGenerate(ctx context.Context, dto InitDto) error
	InitValidate(ctx context.Context, dto InitDto) error
	ValidateBatch(ctx context.Context, batch ProofBatch) error

	// Inference operations
	InferenceHealth(ctx context.Context) (bool, error)
	InferenceUp(ctx context.Context, model string, args []string) error

	// GPU operations
	GetGPUDevices(ctx context.Context) (*GPUDevicesResponse, error)
	GetGPUDriver(ctx context.Context) (*DriverInfo, error)

	// Model management operations
	CheckModelStatus(ctx context.Context, model Model) (*ModelStatusResponse, error)
	DownloadModel(ctx context.Context, model Model) (*DownloadStartResponse, error)
	DeleteModel(ctx context.Context, model Model) (*DeleteResponse, error)
	ListModels(ctx context.Context) (*ModelListResponse, error)
	GetDiskSpace(ctx context.Context) (*DiskSpaceInfo, error)
}

// Ensure Client implements MLNodeClient
var _ MLNodeClient = (*Client)(nil)
