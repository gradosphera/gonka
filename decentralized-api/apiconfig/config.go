package apiconfig

type Config struct {
	Api                 ApiConfig             `koanf:"api"`
	Nodes               []InferenceNodeConfig `koanf:"nodes"`
	NodeConfigIsMerged  bool                  `koanf:"merged_node_config"`
	ChainNode           ChainNodeConfig       `koanf:"chain_node"`
	UpcomingSeed        SeedInfo              `koanf:"upcoming_seed"`
	CurrentSeed         SeedInfo              `koanf:"current_seed"`
	PreviousSeed        SeedInfo              `koanf:"previous_seed"`
	CurrentHeight       int64                 `koanf:"current_height"`
	LastProcessedHeight int64                 `koanf:"last_processed_height"`
	UpgradePlan         UpgradePlan           `koanf:"upgrade_plan"`
	MLNodeKeyConfig     MLNodeKeyConfig       `koanf:"ml_node_key_config"`
	Nats                NatsServerConfig      `koanf:"nats"`
	CurrentNodeVersion  string                `koanf:"current_node_version"`
	LastUsedVersion     string                `koanf:"last_used_version"`
	ValidationParams    ValidationParamsCache `koanf:"validation_params"`
	BandwidthParams     BandwidthParamsCache  `koanf:"bandwidth_params"`
}

type NatsServerConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

type UpgradePlan struct {
	Name        string            `koanf:"name"`
	Height      int64             `koanf:"height"`
	Binaries    map[string]string `koanf:"binaries"`
	NodeVersion string            `koanf:"node_version"`
}

type SeedInfo struct {
	Seed       int64  `koanf:"seed"`
	EpochIndex uint64 `koanf:"epoch_index"`
	Signature  string `koanf:"signature"`
	Claimed    bool   `koanf:"claimed"`
}

type ApiConfig struct {
	Port                  int    `koanf:"port"`
	PoCCallbackUrl        string `koanf:"poc_callback_url"`
	MlGrpcCallbackAddress string `koanf:"ml_grpc_callback_address"`
	PublicUrl             string `koanf:"public_url"`
	PublicServerPort      int    `koanf:"public_server_port"`
	MLServerPort          int    `koanf:"ml_server_port"`
	AdminServerPort       int    `koanf:"admin_server_port"`
	MlGrpcServerPort      int    `koanf:"ml_grpc_server_port"`
	TestMode              bool   `koanf:"test_mode"`
	GenesisAppHashHex     string `koanf:"genesis_app_hash_hex"`
}

type ChainNodeConfig struct {
	Url              string `koanf:"url"`
	IsGenesis        bool   `koanf:"is_genesis"`
	SeedApiUrl       string `koanf:"seed_api_url"`
	AccountPublicKey string `koanf:"account_public_key"`
	SignerKeyName    string `koanf:"signer_key_name"`
	KeyringBackend   string `koanf:"keyring_backend"`
	KeyringDir       string `koanf:"keyring_dir"`
	KeyringPassword  string
}

type MLNodeKeyConfig struct {
	WorkerPublicKey  string `koanf:"worker_public"`
	WorkerPrivateKey string `koanf:"worker_private"`
}

// IF YOU CHANGE ANY OF THESE STRUCTURES BE SURE TO CHANGE HardwareNode proto in inference-chain!!!
type InferenceNodeConfig struct {
	Host             string                 `koanf:"host" json:"host"`
	InferenceSegment string                 `koanf:"inference_segment" json:"inference_segment"`
	InferencePort    int                    `koanf:"inference_port" json:"inference_port"`
	PoCSegment       string                 `koanf:"poc_segment" json:"poc_segment"`
	PoCPort          int                    `koanf:"poc_port" json:"poc_port"`
	Models           map[string]ModelConfig `koanf:"models" json:"models"`
	Id               string                 `koanf:"id" json:"id"`
	MaxConcurrent    int                    `koanf:"max_concurrent" json:"max_concurrent"`
	Hardware         []Hardware             `koanf:"hardware" json:"hardware"`
}

func (n InferenceNodeConfig) DeepCopy() InferenceNodeConfig {
	result := n

	if n.Models != nil {
		result.Models = make(map[string]ModelConfig, len(n.Models))
		for k, v := range n.Models {
			modelCopy := v
			if v.Args != nil {
				modelCopy.Args = make([]string, len(v.Args))
				copy(modelCopy.Args, v.Args)
			}
			result.Models[k] = modelCopy
		}
	}

	if n.Hardware != nil {
		result.Hardware = make([]Hardware, len(n.Hardware))
		copy(result.Hardware, n.Hardware)
	}

	return result
}

type ModelConfig struct {
	Args []string `json:"args"`
}

type Hardware struct {
	Type  string `koanf:"type" json:"type"`
	Count uint32 `koanf:"count" json:"count"`
}

type ValidationParamsCache struct {
	TimestampExpiration int64 `koanf:"timestamp_expiration"`
	TimestampAdvance    int64 `koanf:"timestamp_advance"`
	ExpirationBlocks    int64 `koanf:"expiration_blocks"`
}

type BandwidthParamsCache struct {
	EstimatedLimitsPerBlockKb uint64  `koanf:"estimated_limits_per_block_kb"`
	KbPerInputToken           float64 `koanf:"kb_per_input_token"`
	KbPerOutputToken          float64 `koanf:"kb_per_output_token"`
}
