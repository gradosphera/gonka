package v0_2_5

import (
	"context"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/productscience/inference/x/inference/keeper"
	"github.com/productscience/inference/x/inference/types"
)

// TODO REMOVE ME AFTER TESTS!
var activeParticipantsEpoch0Test = types.ActiveParticipants{
	Participants: []*types.ActiveParticipant{
		{
			ValidatorKey: "tl6U/XtzUsmsUDPoMaColWG72vNqzyXhr6/EP49EvFk=",
		},
	},
	PocStartBlockHeight:  1,
	EffectiveBlockHeight: 1,
	CreatedAtBlockHeight: 1,
}

// The list of participants for epoch 0 is required to verify the entire participant chain from the current epoch back to the genesis epoch. To validate participants, we need to know the consensus public keys of the validators who signed the genesis block.
//
// This was obtained as follows:
//
//  1. We retrieved the list of validator HEX addresses for the genesis block (height = 1) from the LastCommit of the next block (height = 2):
//     curl http://genesis_node__by_your_choice:rpc_port/block?height=2
//
//  2. We retrieved the list of participants from the genesis file:
//     curl -s http://genesis_node__by_your_choice:rpc_port/genesis
//
// 3. Knowing the validator key (consensus key) of each active participant from step 2, we computed its corresponding validator address (HEX) and matched the participants with the validators obtained in step 1.
//
// You can reproduce this process by running:
// python3 inference-chain/scripts/py/match_validators.py <any_genesis_node_rpc> (default is "[http://node2.gonka.ai:26657](http://node2.gonka.ai:26657)")
var activeParticipantsEpoch0Mainnet = types.ActiveParticipants{
	Participants: []*types.ActiveParticipant{
		{
			Index:        "gonka1y2a9p56kv044327uycmqdexl7zs82fs5ryv5le",
			ValidatorKey: "OPwcpfQYOoWDuHKYivRVq5jxrELH0moP5qdznvj3Dps=",
			Weight:       1,
			InferenceUrl: "http://node1.gonka.ai:8000",
		},
		{
			Index:        "gonka1p2lhgng7tcqju7emk989s5fpdr7k2c3ek6h26m",
			ValidatorKey: "LLqBxOz+vD3p7sQsdEhBfrFH2QFMjy3fMasB9yBGSqs=",
			Weight:       1,
			InferenceUrl: "http://47.236.19.22:18000",
		},
		{
			Index:        "gonka1ktl3kkn9l68c9amanu8u4868mcjmtsr5tgzmjk",
			ValidatorKey: "jFC9XywnI7hzIEQ1kmSQf8Q1iuqy861P7vBrqa3LQxs=",
			Weight:       1,
			InferenceUrl: "http://185.216.21.98:8000",
		},
		{
			Index:        "gonka1kx9mca3xm8u8ypzfuhmxey66u0ufxhs7nm6wc5",
			ValidatorKey: "FODVOcIY8RNoGA7WsiNSL6YQ8N4/A5Ox1IyXgc/FmE0=",
			Weight:       1,
			InferenceUrl: "http://node3.gonka.ai:8000",
		},
		{
			Index:        "gonka15p7s7w2hx0y8095lddd4ummm2y0kwpwljk00aq",
			ValidatorKey: "BUWZfCeWI3O+UXcmCbnjacmi0RY0PzX/8aJKdy3rP48=",
			Weight:       1,
			InferenceUrl: "http://36.189.234.197:18026",
		},
		{
			Index:        "gonka1r90m7wlp95zz92eqltys77xyyqkcmz72rc0kv5",
			ValidatorKey: "WDLSFDAjM9OGUER2hmpFivYbaZiXNl8/+2Vq61Z/dDc=",
			Weight:       1,
			InferenceUrl: "http://69.19.136.233:8000",
		},
		{
			Index:        "gonka19fpma3577v3fnk8nxjkvg442ss8hvglxwqgzz6",
			ValidatorKey: "pM9MGrvN6zoLAuA6SKndq2GT/AY8b9tr8PodsnmV4Bk=",
			Weight:       1,
			InferenceUrl: "http://gonka.spv.re:8000",
		},
		{
			Index:        "gonka1dkl4mah5erqggvhqkpc8j3qs5tyuetgdy552cp",
			ValidatorKey: "YHtcky8VaH0qQNhYJkN61RPf83oKWsCPXdaewvDEYLo=",
			Weight:       1,
			InferenceUrl: "http://node2.gonka.ai:8000",
		},
		{
			Index:        "gonka1vhprg9epy683xghp8ddtdlw2y9cycecmm64tje",
			ValidatorKey: "2ykmApZ4pfSMfoREBUDu/vImEYlOym8ymVWOw2wcMQo=",
			Weight:       1,
			InferenceUrl: "http://36.189.234.237:17241",
		},
		{
			Index:        "gonka1d7p03cu2y2yt3vytq9wlfm6tlz0lfhlgv9h82p",
			ValidatorKey: "5QYFI0kdyBPrcld3FfOwoZdynfwN5li0qUbg3zwFK4I=",
			Weight:       1,
			InferenceUrl: "http://47.236.26.199:8000",
		},
		{
			Index:        "gonka1n7njfqnq7z64efe7xma23zu78xex93e04lm52u",
			ValidatorKey: "6BfEgtpNGORi05A9+XTF7yquvV7BKqfOOWcwpD3A8oU=",
			Weight:       1,
			InferenceUrl: "http://93.119.168.58:8000",
		},
	},
	PocStartBlockHeight:  1,
	EffectiveBlockHeight: 1,
	CreatedAtBlockHeight: 1,
	EpochId:              0,
}

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	k keeper.Keeper) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {

		k.LogInfo(fmt.Sprintf("%s - Starting Participants verification upgrade", UpgradeName), types.Upgrades)

		for moduleName, version := range vm {
			fmt.Printf("Module: %s, Version: %d\n", moduleName, version)
		}
		fmt.Printf("OrderMigrations: %v\n", mm.OrderMigrations)

		configurator.RegisterMigration(types.ModuleName, 4, func(ctx sdk.Context) error {
			k.SetActiveParticipants(ctx, activeParticipantsEpoch0Test)
			authorization1 := authztypes.NewGenericAuthorization(sdk.MsgTypeURL(&types.MsgSubmitParticipantsProof{}))
			authorization2 := authztypes.NewGenericAuthorization(sdk.MsgTypeURL(&types.MsgSubmitActiveParticipantsProofData{}))

			participants := k.GetAllParticipant(ctx)
			for _, participant := range participants {
				if participant.ValidatorKey == "" {
					continue
				}

				participantAddr, err := sdk.AccAddressFromBech32(participant.Address)
				if err != nil {
					k.LogError("error getting participant address from string", types.Upgrades, "err", err, "participant", participant.Address)
					continue
				}

				granteesResp, err := k.GranteesByMessageType(ctx, &types.QueryGranteesByMessageTypeRequest{
					GranterAddress: participant.Address,
					MessageTypeUrl: "/inference.inference.MsgStartInference",
				})
				if err != nil {
					k.LogError("error getting grantees", types.Upgrades, "err", err, "granter", participant.Address)
					continue
				}

				for _, grantee := range granteesResp.Grantees {
					granteeAddr, err := sdk.AccAddressFromBech32(grantee.Address)
					if err != nil {
						k.LogError("error getting grantee address from string", types.Upgrades, "err", err, "grantee", grantee.Address)
						continue
					}

					if err := k.AuthzKeeper.SaveGrant(ctx, granteeAddr, participantAddr, authorization1, nil); err != nil {
						k.LogError("error saving grant for authorization1", types.Upgrades, "err", err, "grantee", grantee.Address, "participant", participant.Address)
						continue
					}

					if err := k.AuthzKeeper.SaveGrant(ctx, granteeAddr, participantAddr, authorization2, nil); err != nil {
						k.LogError("error saving grant for authorization2", types.Upgrades, "err", err, "grantee", grantee.Address, "participant", participant.Address)
						continue
					}
				}
			}
			return nil
		})
		if _, ok := vm["capability"]; !ok {
			vm["capability"] = mm.Modules["capability"].(module.HasConsensusVersion).ConsensusVersion()
		}

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
