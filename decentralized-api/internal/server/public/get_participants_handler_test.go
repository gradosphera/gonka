package public

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"decentralized-api/cosmosclient"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/labstack/echo/v4"
	"github.com/productscience/inference/x/inference/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type fakeQueryServer struct {
	types.UnimplementedQueryServer

	firstPageParticipants  []types.Participant
	secondPageParticipants []types.Participant
	receivedHeights        []string
}

func (f *fakeQueryServer) ParticipantAll(ctx context.Context, req *types.QueryAllParticipantRequest) (*types.QueryAllParticipantResponse, error) {
	if req.Pagination == nil || len(req.Pagination.Key) == 0 {
		md := metadata.Pairs(grpctypes.GRPCBlockHeightHeader, "12345")
		_ = grpc.SendHeader(ctx, md)
		return &types.QueryAllParticipantResponse{
			Participant: f.firstPageParticipants,
			Pagination:  &query.PageResponse{NextKey: []byte("next"), Total: uint64(len(f.firstPageParticipants) + len(f.secondPageParticipants))},
		}, nil
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		f.receivedHeights = append(f.receivedHeights, md.Get(grpctypes.GRPCBlockHeightHeader)...)
	}
	return &types.QueryAllParticipantResponse{
		Participant: f.secondPageParticipants,
		Pagination:  &query.PageResponse{NextKey: nil, Total: uint64(len(f.firstPageParticipants) + len(f.secondPageParticipants))},
	}, nil
}

func startBufGRPCServer(t *testing.T, srv types.QueryServer) (*grpc.ClientConn, func()) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	types.RegisterQueryServer(server, srv)
	go func() { _ = server.Serve(listener) }()
	dialer := func(context.Context, string) (net.Conn, error) { return listener.Dial() }
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	require.NoError(t, err)
	cleanup := func() { server.Stop(); _ = listener.Close(); _ = conn.Close() }
	return conn, cleanup
}

func TestGetAllParticipants_PaginationAndPinnedHeight(t *testing.T) {
	first := make([]types.Participant, 100)
	second := make([]types.Participant, 50)
	for i := 0; i < 100; i++ {
		first[i] = types.Participant{Address: fmt.Sprintf("addr%03d", i), InferenceUrl: "http://node", CoinBalance: int64(i), Weight: int32(i)}
	}
	for i := 0; i < 50; i++ {
		second[i] = types.Participant{Address: fmt.Sprintf("addr%03d", 100+i), InferenceUrl: "http://node", CoinBalance: int64(100 + i), Weight: int32(100 + i)}
	}

	fq := &fakeQueryServer{firstPageParticipants: first, secondPageParticipants: second}
	conn, cleanup := startBufGRPCServer(t, fq)
	defer cleanup()

	mc := &cosmosclient.MockCosmosMessageClient{}
	mc.On("NewInferenceQueryClient").Return(types.NewQueryClient(conn))
	mc.On("BankBalances", mock.Anything, mock.AnythingOfType("string")).Return([]sdk.Coin{sdk.NewInt64Coin("ngonka", 42)}, nil)

	e := echo.New()
	s := &Server{e: e, recorder: mc}
	req := httptest.NewRequest(http.MethodGet, "/participants", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, s.getAllParticipants(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var dto ParticipantsDto
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	require.Equal(t, int64(12345), dto.BlockHeight)
	require.Len(t, dto.Participants, 150)
	require.Equal(t, "addr000", dto.Participants[0].Id)
	require.Equal(t, int64(42), dto.Participants[0].Balance)
	last := dto.Participants[len(dto.Participants)-1]
	require.Equal(t, "addr149", last.Id)

	require.GreaterOrEqual(t, len(fq.receivedHeights), 1)
	require.Equal(t, "12345", fq.receivedHeights[0])

	mc.AssertExpectations(t)
}

// TODO: do not delete test: used to check state on archive node
/*func TestArchiveNode(t *testing.T) {
	const epoch = 1
	rpcClient, err := cosmos_client.NewRpcClient("http://204.12.168.157:26657")
	assert.NoError(t, err)

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)

	cdc := codec.NewProtoCodec(interfaceRegistry)
	dataKey := types.ActiveParticipantsFullKey(epoch)
	result, err := cosmos_client.QueryByKey(rpcClient, "inference", dataKey)
	assert.NoError(t, err)

	var activeParticipants types.ActiveParticipants
	err = cdc.Unmarshal(result.Response.Value, &activeParticipants)
	assert.NoError(t, err)

	blockHeight := activeParticipants.CreatedAtBlockHeight
	result, err = cosmos_client.QueryByKeyWithOptions(rpcClient, "inference", dataKey, blockHeight, true)

	fmt.Printf("%+v", result)
}
*/
