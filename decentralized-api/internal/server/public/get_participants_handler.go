package public

import (
	"context"
	cosmos_client "decentralized-api/cosmosclient"
	"decentralized-api/internal/utils"
	"decentralized-api/logging"
	"decentralized-api/merkleproof"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	comettypes "github.com/cometbft/cometbft/types"

	"github.com/cometbft/cometbft/crypto/tmhash"
	rpcclient "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/labstack/echo/v4"
	"github.com/productscience/inference/x/inference/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func (s *Server) getInferenceParticipantByAddress(c echo.Context) error {
	address := c.Param("address")
	if address == "" {
		return ErrAddressRequired
	}

	logging.Debug("GET inference participant", types.Inferences, "address", address)

	queryClient := s.recorder.NewInferenceQueryClient()
	response, err := queryClient.InferenceParticipant(c.Request().Context(), &types.QueryInferenceParticipantRequest{
		Address: address,
	})
	if err != nil {
		logging.Error("Failed to get inference participant", types.Inferences, "address", address, "error", err)
		return err
	}

	if response == nil {
		logging.Error("Inference participant not found", types.Inferences, "address", address)
		return ErrInferenceParticipantNotFound
	}

	return c.JSON(http.StatusOK, response)
}

func (s *Server) getParticipantsByEpoch(c echo.Context) error {
	epochParam := c.Param("epoch")
	if epochParam == "" {
		return ErrInvalidEpochId
	}

	rpcClient, err := cosmos_client.NewRpcClient(s.configManager.GetChainNodeConfig().Url)
	if err != nil {
		logging.Error("Failed to create rpc client", types.System, "error", err)
		return err
	}

	activeParticipants, err := utils.QueryActiveParticipants(rpcClient, s.recorder.NewInferenceQueryClient())(context.Background(), epochParam)
	if err != nil {
		logging.Error("Failed to query active participants. Outer", types.Participants, "error", err)
		return err
	}

	return c.JSON(http.StatusOK, activeParticipants)
}

func (s *Server) getAllParticipants(ctx echo.Context) error {
	queryClient := s.recorder.NewInferenceQueryClient()
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
				if pBalance == 0 {
					logging.Debug("Participant has no balance", types.Participants, "address", p.Address)
				}
			} else {
				logging.Warn("Failed to get balance for participant", types.Participants, "address", p.Address, "error", err)
			}
			participants = append(participants, ParticipantDto{
				Id:          p.Address,
				Url:         p.InferenceUrl,
				CoinsOwed:   p.CoinBalance,
				Balance:     pBalance,
				VotingPower: int64(p.Weight),
			})
		}
		if resp.Pagination != nil {
			nextKey = resp.Pagination.NextKey
		}
	}

	// Process remaining pages
	for len(nextKey) > 0 {
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
				if pBalance == 0 {
					logging.Debug("Participant has no balance", types.Participants, "address", p.Address)
				}
			} else {
				logging.Warn("Failed to get balance for participant", types.Participants, "address", p.Address, "error", err)
			}
			participants = append(participants, ParticipantDto{
				Id:          p.Address,
				Url:         p.InferenceUrl,
				CoinsOwed:   p.CoinBalance,
				Balance:     pBalance,
				VotingPower: int64(p.Weight),
			})
		}

		if resp.Pagination == nil || len(resp.Pagination.NextKey) == 0 {
			break
		}
		nextKey = resp.Pagination.NextKey
	}

	return ctx.JSON(http.StatusOK, &ParticipantsDto{
		Participants: participants,
		BlockHeight:  blockHeight,
	})
}
