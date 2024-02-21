package feechecker_test

import (
	"math"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/stretchr/testify/suite"

	bandtesting "github.com/bandprotocol/chain/v2/testing"
	"github.com/bandprotocol/chain/v2/x/globalfee/feechecker"
	"github.com/bandprotocol/chain/v2/x/oracle/types"
)

var (
	BasicCalldata = []byte("BASIC_CALLDATA")
	BasicClientID = "BASIC_CLIENT_ID"
)

type StubTx struct {
	sdk.Tx
	sdk.FeeTx
	Msgs      []sdk.Msg
	GasPrices sdk.DecCoins
}

func (st *StubTx) GetMsgs() []sdk.Msg {
	return st.Msgs
}

func (st *StubTx) ValidateBasic() error {
	return nil
}

func (st *StubTx) GetGas() uint64 {
	return 1000000
}

func (st *StubTx) GetFee() sdk.Coins {
	fees := make(sdk.Coins, len(st.GasPrices))

	// Determine the fees by multiplying each gas prices
	glDec := sdk.NewDec(int64(st.GetGas()))
	for i, gp := range st.GasPrices {
		fee := gp.Amount.Mul(glDec)
		fees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
	}

	return fees
}

type FeeCheckerTestSuite struct {
	suite.Suite
	FeeChecker feechecker.FeeChecker
	ctx        sdk.Context
	requestID  types.RequestID
}

func (suite *FeeCheckerTestSuite) SetupTest() {
	app, ctx := bandtesting.CreateTestApp(suite.T(), true)

	suite.ctx = ctx.WithBlockHeight(999).
		WithIsCheckTx(true).
		WithMinGasPrices(sdk.DecCoins{{Denom: "uband", Amount: sdk.NewDecWithPrec(1, 4)}})

	err := app.OracleKeeper.GrantReporter(suite.ctx, bandtesting.Validators[0].ValAddress, bandtesting.Alice.Address)
	suite.Require().NoError(err)

	req := types.NewRequest(
		1,
		BasicCalldata,
		[]sdk.ValAddress{bandtesting.Validators[0].ValAddress},
		1,
		1,
		bandtesting.ParseTime(0),
		"",
		nil,
		nil,
		0,
	)
	suite.requestID = app.OracleKeeper.AddRequest(suite.ctx, req)

	suite.FeeChecker = feechecker.NewFeeChecker(
		&app.OracleKeeper,
		&app.GlobalfeeKeeper,
		app.StakingKeeper,
	)
}

func (suite *FeeCheckerTestSuite) TestValidRawReport() {
	msgs := []sdk.Msg{
		types.NewMsgReportData(suite.requestID, []types.RawReport{}, bandtesting.Validators[0].ValAddress),
	}
	stubTx := &StubTx{Msgs: msgs}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().True(isReportTx)

	// test - check tx fee with min gas prices
	fee, priority, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().NoError(err)
	suite.Require().Equal(sdk.Coins{}, fee)
	suite.Require().Equal(int64(math.MaxInt64), priority)
}

func (suite *FeeCheckerTestSuite) TestNotValidRawReport() {
	msgs := []sdk.Msg{types.NewMsgReportData(1, []types.RawReport{}, bandtesting.Alice.ValAddress)}
	stubTx := &StubTx{Msgs: msgs}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().False(isReportTx)

	// test - check tx fee with min gas prices
	_, _, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().Error(err)
}

func (suite *FeeCheckerTestSuite) TestValidReport() {
	reportMsgs := []sdk.Msg{
		types.NewMsgReportData(suite.requestID, []types.RawReport{}, bandtesting.Validators[0].ValAddress),
	}
	authzMsg := authz.NewMsgExec(bandtesting.Alice.Address, reportMsgs)
	stubTx := &StubTx{Msgs: []sdk.Msg{&authzMsg}}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().True(isReportTx)

	// test - check tx fee with min gas prices
	fee, priority, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().NoError(err)
	suite.Require().Equal(sdk.Coins{}, fee)
	suite.Require().Equal(int64(math.MaxInt64), priority)
}

func (suite *FeeCheckerTestSuite) TestNoAuthzReport() {
	reportMsgs := []sdk.Msg{
		types.NewMsgReportData(suite.requestID, []types.RawReport{}, bandtesting.Validators[0].ValAddress),
	}
	authzMsg := authz.NewMsgExec(bandtesting.Bob.Address, reportMsgs)
	stubTx := &StubTx{Msgs: []sdk.Msg{&authzMsg}, GasPrices: sdk.NewDecCoins(sdk.NewDecCoin("uband", sdk.NewInt(1)))}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().False(isReportTx)

	// test - check tx fee with min gas prices
	_, _, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().NoError(err)
}

func (suite *FeeCheckerTestSuite) TestNotValidReport() {
	reportMsgs := []sdk.Msg{
		types.NewMsgReportData(suite.requestID+1, []types.RawReport{}, bandtesting.Validators[0].ValAddress),
	}
	authzMsg := authz.NewMsgExec(bandtesting.Alice.Address, reportMsgs)
	stubTx := &StubTx{Msgs: []sdk.Msg{&authzMsg}}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().False(isReportTx)

	// test - check tx fee with min gas prices
	_, _, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().Error(err)
}

func (suite *FeeCheckerTestSuite) TestNotReportMsg() {
	requestMsg := types.NewMsgRequestData(
		1,
		BasicCalldata,
		1,
		1,
		BasicClientID,
		bandtesting.Coins100000000uband,
		bandtesting.TestDefaultPrepareGas,
		bandtesting.TestDefaultExecuteGas,
		bandtesting.FeePayer.Address,
	)
	stubTx := &StubTx{
		Msgs: []sdk.Msg{requestMsg},
		GasPrices: sdk.NewDecCoins(
			sdk.NewDecCoinFromDec("uaaaa", sdk.NewDecWithPrec(100, 3)),
			sdk.NewDecCoinFromDec("uaaab", sdk.NewDecWithPrec(1, 3)),
			sdk.NewDecCoinFromDec("uaaac", sdk.NewDecWithPrec(0, 3)),
			sdk.NewDecCoinFromDec("uband", sdk.NewDecWithPrec(3, 3)),
			sdk.NewDecCoinFromDec("uccca", sdk.NewDecWithPrec(0, 3)),
			sdk.NewDecCoinFromDec("ucccb", sdk.NewDecWithPrec(1, 3)),
			sdk.NewDecCoinFromDec("ucccc", sdk.NewDecWithPrec(100, 3)),
		),
	}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().False(isReportTx)

	// test - check tx fee with min gas prices
	fee, priority, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().NoError(err)
	suite.Require().Equal(stubTx.GetFee(), fee)
	suite.Require().Equal(int64(30), priority)
}

func (suite *FeeCheckerTestSuite) TestReportMsgAndOthersTypeMsgInTheSameAuthzMsgs() {
	reportMsg := types.NewMsgReportData(suite.requestID, []types.RawReport{}, bandtesting.Validators[0].ValAddress)
	requestMsg := types.NewMsgRequestData(
		1,
		BasicCalldata,
		1,
		1,
		BasicClientID,
		bandtesting.Coins100000000uband,
		bandtesting.TestDefaultPrepareGas,
		bandtesting.TestDefaultExecuteGas,
		bandtesting.FeePayer.Address,
	)
	msgs := []sdk.Msg{reportMsg, requestMsg}
	authzMsg := authz.NewMsgExec(bandtesting.Alice.Address, msgs)
	stubTx := &StubTx{Msgs: []sdk.Msg{&authzMsg}, GasPrices: sdk.NewDecCoins(sdk.NewDecCoin("uband", sdk.NewInt(1)))}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().False(isReportTx)

	// test - check tx fee with min gas prices
	fee, priority, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().NoError(err)
	suite.Require().Equal(stubTx.GetFee(), fee)
	suite.Require().Equal(int64(10000), priority)
}

func (suite *FeeCheckerTestSuite) TestReportMsgAndOthersTypeMsgInTheSameTx() {
	reportMsg := types.NewMsgReportData(suite.requestID, []types.RawReport{}, bandtesting.Validators[0].ValAddress)
	requestMsg := types.NewMsgRequestData(
		1,
		BasicCalldata,
		1,
		1,
		BasicClientID,
		bandtesting.Coins100000000uband,
		bandtesting.TestDefaultPrepareGas,
		bandtesting.TestDefaultExecuteGas,
		bandtesting.FeePayer.Address,
	)
	stubTx := &StubTx{
		Msgs:      []sdk.Msg{reportMsg, requestMsg},
		GasPrices: sdk.NewDecCoins(sdk.NewDecCoin("uband", sdk.NewInt(1))),
	}

	// test - check report tx
	isReportTx := suite.FeeChecker.CheckReportTx(suite.ctx, stubTx)
	suite.Require().False(isReportTx)

	// test - check tx fee with min gas prices
	fee, priority, err := suite.FeeChecker.CheckTxFeeWithMinGasPrices(suite.ctx, stubTx)
	suite.Require().NoError(err)
	suite.Require().Equal(stubTx.GetFee(), fee)
	suite.Require().Equal(int64(10000), priority)
}

func (suite *FeeCheckerTestSuite) TestGetBondDenom() {
	denom := suite.FeeChecker.GetBondDenom(suite.ctx)
	suite.Require().Equal("uband", denom)
}

func (suite *FeeCheckerTestSuite) TestDefaultZeroGlobalFee() {
	coins, err := suite.FeeChecker.DefaultZeroGlobalFee(suite.ctx)

	suite.Require().Equal(1, len(coins))
	suite.Require().Equal("uband", coins[0].Denom)
	suite.Require().Equal(sdk.NewDec(0), coins[0].Amount)
	suite.Require().NoError(err)
}

func TestFeeCheckerTestSuite(t *testing.T) {
	suite.Run(t, new(FeeCheckerTestSuite))
}
