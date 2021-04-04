package keeper

import (
	"context"
	"fmt"

	"github.com/bandprotocol/chain/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CustomQuery interface {
	CustomQuery(path string) (result []byte, match bool, err error)
}

type queryServer struct {
	Keeper
	custom CustomQuery
}

// NewQuerierServerImpl returns an implementation of the oracle QueryServer interface
// for the provided Keeper.
func NewQuerierServerImpl(keeper Keeper, custom CustomQuery) types.QueryServer {
	return &queryServer{
		Keeper: keeper,
		custom: custom,
	}
}

var _ types.QueryServer = queryServer{}

// Counts queries the number of data sources, oracle scripts, and requests.
func (k queryServer) Counts(c context.Context, req *types.QueryCountsRequest) (*types.QueryCountsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	return &types.QueryCountsResponse{
			DataSourceCount:   k.GetDataSourceCount(ctx),
			OracleScriptCount: k.GetOracleScriptCount(ctx),
			RequestCount:      k.GetRequestCount(ctx)},
		nil
}

// Data queries the data source or oracle script script for given file hash.
func (k queryServer) Data(c context.Context, req *types.QueryDataRequest) (*types.QueryDataResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	data, err := k.fileCache.GetFile(req.DataHash)
	if err != nil {
		return nil, err
	}
	return &types.QueryDataResponse{Data: data}, nil
}

// DataSource queries data source info for given data source id.
func (k queryServer) DataSource(c context.Context, req *types.QueryDataSourceRequest) (*types.QueryDataSourceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	ds, err := k.GetDataSource(ctx, types.DataSourceID(req.DataSourceId))
	if err != nil {
		return nil, err
	}
	return &types.QueryDataSourceResponse{DataSource: &ds}, nil
}

// OracleScript queries oracle script info for given oracle script id.
func (k queryServer) OracleScript(c context.Context, req *types.QueryOracleScriptRequest) (*types.QueryOracleScriptResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	os, err := k.GetOracleScript(ctx, types.OracleScriptID(req.OracleScriptId))
	if err != nil {
		return nil, err
	}
	return &types.QueryOracleScriptResponse{OracleScript: &os}, nil
}

// Request queries request info for given request id.
func (k queryServer) Request(c context.Context, req *types.QueryRequestRequest) (*types.QueryRequestResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	r, err := k.GetResult(ctx, types.RequestID(req.RequestId))
	if err != nil {
		return nil, err
	}
	// TODO: Define specification on this endpoint (For test only)
	return &types.QueryRequestResponse{&types.OracleRequestPacketData{}, &types.OracleResponsePacketData{Result: r.Result}}, nil
}

// Validator queries oracle info of validator for given validator
// address.
func (k queryServer) Validator(c context.Context, req *types.QueryValidatorRequest) (*types.QueryValidatorResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	val, err := sdk.ValAddressFromBech32(req.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	status := k.GetValidatorStatus(ctx, val)
	if err != nil {
		return nil, err
	}
	return &types.QueryValidatorResponse{Status: &status}, nil
}

// Reporters queries all reporters of a given validator address.
func (k queryServer) Reporters(c context.Context, req *types.QueryReportersRequest) (*types.QueryReportersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	val, err := sdk.ValAddressFromBech32(req.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	reps := k.GetReporters(ctx, val)
	reporters := make([]string, len(reps))
	for idx, rep := range reps {
		reporters[idx] = rep.String()
	}
	return &types.QueryReportersResponse{Reporter: reporters}, nil
}

// ActiveValidators queries all active oracle validators.
func (k queryServer) ActiveValidators(c context.Context, req *types.QueryActiveValidatorsRequest) (*types.QueryActiveValidatorsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	vals := []types.QueryActiveValidatorResult{}
	k.stakingKeeper.IterateBondedValidatorsByPower(ctx,
		func(idx int64, val stakingtypes.ValidatorI) (stop bool) {
			if k.GetValidatorStatus(ctx, val.GetOperator()).IsActive {
				vals = append(vals, types.QueryActiveValidatorResult{
					Address: val.GetOperator(),
					Power:   val.GetTokens().Uint64(),
				})
			}
			return false
		})
	return &types.QueryActiveValidatorsResponse{Count: int64(len(vals))}, nil
}

// Params queries the oracle parameters.
func (k queryServer) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

// RequestSearch queries the latest request that match the given input.
func (k queryServer) RequestSearch(c context.Context, req *types.QueryRequestSearchRequest) (*types.QueryRequestSearchResponse, error) {
	res, match, err := k.custom.CustomQuery(fmt.Sprintf("latest_request/%s", req.Calldata))
	if !match {
		return nil, status.Error(codes.Internal, "Not support on this version")
	}
	if err != nil {
		return nil, err
	}
	return &types.QueryRequestSearchResponse{Test: string(res)}, nil
}

// RequestPrice queries the latest price on standard price reference oracle
// script.
func (k queryServer) RequestPrice(c context.Context, req *types.QueryRequestPriceRequest) (*types.QueryRequestPriceResponse, error) {
	return &types.QueryRequestPriceResponse{}, nil
}
