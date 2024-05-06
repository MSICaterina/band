package emitter

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	solomachinetypes "github.com/cosmos/ibc-go/v7/modules/light-clients/06-solomachine"
	ibctmtypes "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"

	"github.com/bandprotocol/chain/v2/hooks/common"
)

func getChainId(clientState exported.ClientState) string {
	switch cs := clientState.(type) {
	case *ibctmtypes.ClientState:
		return cs.ChainId
	case *solomachinetypes.ClientState:
		return "solo-machine"
	default:
		return "unknown"
	}
}

func (h *Hook) getChainIdFromClientId(ctx sdk.Context, clientId string) string {
	clientState, _ := h.clientKeeper.GetClientState(ctx, clientId)
	return getChainId(clientState)
}

func (h *Hook) handleMsgCreatClient(ctx sdk.Context, msg *types.MsgCreateClient, detail common.JsDict) {
	// h.clientkeeper.GetClientConsensusState(ctx,msg.ClientState)
	clientState, _ := types.UnpackClientState(msg.ClientState)
	chainId := getChainId(clientState)
	h.Write("SET_COUNTERPARTY_CHAIN", common.JsDict{
		"chain_id": chainId,
	})
	detail["chain_id"] = chainId
}
