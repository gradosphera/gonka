package tx_manager

import (
	"context"
	"decentralized-api/apiconfig"
	"decentralized-api/internal/nats/client"
	natssrv "decentralized-api/internal/nats/server"
	"encoding/base64"
	"encoding/json"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/uuid"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient/mocks"
	"github.com/productscience/inference/api/inference/inference"
	testutil "github.com/productscience/inference/testutil/cosmoclient"
	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPack_Unpack_Msg(t *testing.T) {
	const (
		network = "cosmos"

		accountName = "cosmosaccount"
		mnemonic    = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
		passphrase  = "testpass"
	)

	rpc := mocks.NewRPCClient(t)
	client := testutil.NewMockClient(t, rpc, network, accountName, mnemonic, passphrase)

	rawTx := &inference.MsgFinishInference{
		Creator:              "some_address",
		InferenceId:          uuid.New().String(),
		ResponseHash:         "some_hash",
		ResponsePayload:      "resp",
		PromptTokenCount:     10,
		CompletionTokenCount: 20,
		ExecutedBy:           "executor",
	}

	bz, err := client.Context().Codec.MarshalInterfaceJSON(rawTx)
	assert.NoError(t, err)

	timeout := getTimestamp(time.Now().UnixNano(), time.Second)
	b, err := json.Marshal(&txToSend{TxInfo: txInfo{RawTx: bz, Timeout: timeout}})
	assert.NoError(t, err)

	var tx txToSend
	err = json.Unmarshal(b, &tx)
	assert.NoError(t, err)

	var unpackedAny codectypes.Any
	err = client.Context().Codec.UnmarshalJSON(tx.TxInfo.RawTx, &unpackedAny)
	assert.NoError(t, err)

	var unmarshalledRawTx sdk.Msg
	err = client.Context().Codec.UnpackAny(&unpackedAny, &unmarshalledRawTx)
	assert.NoError(t, err)

	result := unmarshalledRawTx.(*types.MsgFinishInference)

	assert.Equal(t, rawTx.InferenceId, result.InferenceId)
	assert.Equal(t, rawTx.Creator, result.Creator)
	assert.Equal(t, rawTx.ResponseHash, result.ResponseHash)
	assert.Equal(t, rawTx.ResponsePayload, result.ResponsePayload)
	assert.Equal(t, rawTx.PromptTokenCount, result.PromptTokenCount)
	assert.Equal(t, rawTx.CompletionTokenCount, result.CompletionTokenCount)
	assert.Equal(t, rawTx.ExecutedBy, result.ExecutedBy)
}

func TestTxManagerOnChainHalt(t *testing.T) {
	const addr = "gonka1af8s0906kzuj8kyf69zn5n77jcrg9ttqhg4jwy"
	config := apiconfig.NatsServerConfig{
		Host: "0.0.0.0",
		Port: 4111,
	}

	srv := natssrv.NewServer(config)

	err := srv.Start()
	assert.NoError(t, err)

	ctx := context.Background()
	cosmoclient, err := cosmosclient.New(
		ctx,
		cosmosclient.WithAddressPrefix("gonka"),
		cosmosclient.WithKeyringServiceName("inferenced"),
		cosmosclient.WithNodeAddress("http://localhost:26657"),
		cosmosclient.WithKeyringDir("/home/zb/jobs/productai/code/gonka/local-test-net/prod-local/genesis"),
		cosmosclient.WithGasPrices("0ngonka"),
		cosmosclient.WithFees("0ngonka"),
		cosmosclient.WithGas("auto"),
		cosmosclient.WithGasAdjustment(5),
	)

	natsConn, err := client.ConnectToNats(config.Host, config.Port, "tx_manager")
	assert.NoError(t, err)

	acc, err := cosmoclient.Account(addr)
	assert.NoError(t, err)

	pubKeyBytes, err := base64.StdEncoding.DecodeString("AuwJlMZwZb8PhFKJbll8l6COhkBisgpkF80AX4i1BY6b")
	assert.NoError(t, err)

	accountKey := secp256k1.PubKey{Key: pubKeyBytes}

	txManager, err := StartTxManager(
		ctx,
		&cosmoclient,
		&apiconfig.ApiAccount{
			AccountKey:    &accountKey,
			SignerAccount: &acc,
			AddressPrefix: "gonka",
		},
		5*time.Second,
		natsConn, addr,
	)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = txManager.SendTransactionAsyncWithRetry(&types.MsgStartInference{
		Creator:     addr,
		AssignedTo:  addr,
		InferenceId: uuid.New().String(),
		RequestedBy: addr,
	})
	assert.NoError(t, err)
	<-ctx.Done()
}
