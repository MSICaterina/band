package proof

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bandprotocol/chain/x/oracle/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/ethereum/go-ethereum/accounts/abi"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

type JsonRequestsCountProof struct {
	BlockHeight     uint64             `json:"blockHeigh"`
	CountProof      RequestsCountProof `json:"countProof"`
	BlockRelayProof BlockRelayProof    `json:"blockRelayProof"`
}

type CountProof struct {
	JsonProof     JsonRequestsCountProof `json:"jsonProof"`
	EVMProofBytes tmbytes.HexBytes       `json:"evmProofBytes"`
}

func GetRequestsCountProofHandlerFn(cliCtx client.Context, route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
		if !ok {
			return
		}
		height := &ctx.Height
		if ctx.Height == 0 {
			height = nil
		}

		commit, err := ctx.Client.Commit(context.Background(), height)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		value, iavlEp, multiStoreEp, err := getProofsByKey(
			ctx,
			types.RequestCountStoreKey,
			rpcclient.ABCIQueryOptions{Height: commit.Height - 1, Prove: true},
		)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Produce block relay proof
		signatures, err := GetSignaturesAndPrefix(&commit.SignedHeader)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		blockRelay := BlockRelayProof{
			MultiStoreProof:        GetMultiStoreProof(multiStoreEp),
			BlockHeaderMerkleParts: GetBlockHeaderMerkleParts(ctx.LegacyAmino, commit.Header),
			Signatures:             signatures,
		}

		// Parse requests count
		var rs int64
		ctx.LegacyAmino.MustUnmarshalBinaryLengthPrefixed(value, &rs)

		requestsCountProof := RequestsCountProof{
			Count:       uint64(rs),
			Prefix:      iavlEp.Leaf.Prefix,
			MerklePaths: GetIAVLMerklePaths(iavlEp),
		}

		// Calculate byte for proofbytes
		var relayAndVerifyCountArguments abi.Arguments
		format := `[{"type":"bytes"},{"type":"bytes"}]`
		err = json.Unmarshal([]byte(format), &relayAndVerifyCountArguments)
		if err != nil {
			panic(err)
		}

		blockRelayBytes, err := blockRelay.encodeToEthData()
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		requestsCountBytes, err := requestsCountProof.encodeToEthData(uint64(commit.Height))
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		evmProofBytes, err := relayAndVerifyCountArguments.Pack(blockRelayBytes, requestsCountBytes)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		rest.PostProcessResponse(w, ctx, CountProof{
			JsonProof: JsonRequestsCountProof{
				BlockHeight:     uint64(commit.Height - 1),
				CountProof:      requestsCountProof,
				BlockRelayProof: blockRelay,
			},
			EVMProofBytes: evmProofBytes,
		})
	}
}
