package price

import (
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/syndtr/goleveldb/leveldb"
	abci "github.com/tendermint/tendermint/abci/types"

	band "github.com/bandprotocol/chain/v2/app"
	"github.com/bandprotocol/chain/v2/hooks/common"
	"github.com/bandprotocol/chain/v2/pkg/obi"
	"github.com/bandprotocol/chain/v2/x/oracle/keeper"
	"github.com/bandprotocol/chain/v2/x/oracle/types"
)

// Hook uses levelDB to store the latest price of standard price reference.
type Hook struct {
	cdc             codec.Codec
	stdOs           map[types.OracleScriptID]bool
	oracleKeeper    keeper.Keeper
	db              *leveldb.DB
	defaultAskCount uint64
	defaultMinCount uint64
}

var _ band.Hook = &Hook{}

// NewHook creates a price hook instance that will be added in Band App.
func NewHook(cdc codec.Codec, oracleKeeper keeper.Keeper, oids []types.OracleScriptID, priceDBDir string, defaultAskCount uint64, defaultMinCount uint64) *Hook {
	stdOs := make(map[types.OracleScriptID]bool)
	for _, oid := range oids {
		stdOs[oid] = true
	}
	db, err := leveldb.OpenFile(priceDBDir, nil)
	if err != nil {
		panic(err)
	}
	return &Hook{
		cdc:             cdc,
		stdOs:           stdOs,
		oracleKeeper:    oracleKeeper,
		db:              db,
		defaultAskCount: defaultAskCount,
		defaultMinCount: defaultMinCount,
	}
}

// AfterInitChain specify actions need to do after chain initialization (app.Hook interface).
func (h *Hook) AfterInitChain(ctx sdk.Context, req abci.RequestInitChain, res abci.ResponseInitChain) {
}

// AfterBeginBlock specify actions need to do after begin block period (app.Hook interface).
func (h *Hook) AfterBeginBlock(ctx sdk.Context, req abci.RequestBeginBlock, res abci.ResponseBeginBlock) {
}

// AfterDeliverTx specify actions need to do after transaction has been processed (app.Hook interface).
func (h *Hook) AfterDeliverTx(ctx sdk.Context, req abci.RequestDeliverTx, res abci.ResponseDeliverTx) {
}

// AfterEndBlock specify actions need to do after end block period (app.Hook interface).
func (h *Hook) AfterEndBlock(ctx sdk.Context, req abci.RequestEndBlock, res abci.ResponseEndBlock) {
	for _, event := range res.Events {
		events := sdk.StringifyEvents([]abci.Event{event})
		evMap := common.ParseEvents(events)
		switch event.Type {
		case types.EventTypeResolve:
			reqID := types.RequestID(common.Atoi(evMap[types.EventTypeResolve+"."+types.AttributeKeyID][0]))
			result := h.oracleKeeper.MustGetResult(ctx, reqID)

			if result.ResolveStatus == types.RESOLVE_STATUS_SUCCESS {
				// check whether this result should be stored to database
				if h.stdOs[result.OracleScriptID] {
					var input Input
					var output Output
					obi.MustDecode(result.Calldata, &input)
					obi.MustDecode(result.Result, &output)
					for idx, symbol := range input.Symbols {
						price := types.PriceResult{
							Symbol:      symbol,
							Multiplier:  input.Multiplier,
							Px:          output.Rates[idx],
							RequestID:   result.RequestID,
							ResolveTime: result.ResolveTime,
						}
						err := h.db.Put([]byte(fmt.Sprintf("%d,%d,%s", result.AskCount, result.MinCount, symbol)),
							h.cdc.MustMarshal(&price), nil)
						if err != nil {
							panic(err)
						}
					}
				}
			}
		}
	}
}

// ApplyQuery catch the custom query that matches specific paths (app.Hook interface).
func (h *Hook) ApplyQuery(req abci.RequestQuery) (res abci.ResponseQuery, stop bool) {
	switch req.Path {
	case "/oracle.v1.Query/RequestPrice":
		var request types.QueryRequestPriceRequest
		if err := h.cdc.Unmarshal(req.Data, &request); err != nil {
			return sdkerrors.QueryResult(sdkerrors.Wrapf(sdkerrors.ErrLogic, "unable to parse request of RequestPrice query: %s", err)), true
		}

		var response types.QueryRequestPriceResponse
		for _, symbol := range request.Symbols {
			var priceResult types.PriceResult

			if request.AskCount == 0 && request.MinCount == 0 {
				request.AskCount = h.defaultAskCount
				request.MinCount = h.defaultMinCount
			}
			bz, err := h.db.Get([]byte(fmt.Sprintf("%d,%d,%s", request.AskCount, request.MinCount, symbol)), nil)
			if err != nil {
				if errors.Is(err, leveldb.ErrNotFound) {
					return sdkerrors.QueryResult(sdkerrors.Wrapf(
						sdkerrors.ErrKeyNotFound,
						"price not found for %s with %d/%d counts",
						symbol,
						request.AskCount,
						request.MinCount,
					)), true
				}
				return sdkerrors.QueryResult(
					sdkerrors.Wrapf(sdkerrors.ErrLogic,
						"unable to get price of %s with %d/%d counts",
						symbol,
						request.AskCount,
						request.MinCount,
					),
				), true
			}

			h.cdc.MustUnmarshal(bz, &priceResult)
			response.PriceResults = append(response.PriceResults, &priceResult)
		}

		bz := h.cdc.MustMarshal(&response)

		return common.QueryResultSuccess(bz, req.Height), true
	default:
		return
	}
}

// BeforeCommit specify actions need to do before commit block (app.Hook interface).
func (h *Hook) BeforeCommit() {}
