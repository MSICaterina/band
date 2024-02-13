package main

import (
	"errors"

	httpclient "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runCmd(c *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"r"},
		Short:   "Run the oracle process",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.ChainID == "" {
				return errors.New("chain ID must not be empty")
			}
			keys, err := keybase.List()
			if err != nil {
				return err
			}
			if len(keys) == 0 {
				return errors.New("no key available")
			}
			c.keys = make(chan keyring.Record, len(keys))
			for _, key := range keys {
				c.keys <- *key
			}
			c.gasPrices, err = sdk.ParseDecCoins(cfg.GasPrices)
			if err != nil {
				return err
			}
			c.client, err = httpclient.New(cfg.NodeURI, "/websocket")
			if err != nil {
				return err
			}
			c.amount = sdk.NewCoins(sdk.NewCoin("uband", sdk.NewInt(cfg.Amount)))
			r := gin.Default()

			// add cors
			r.Use(func(c *gin.Context) {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().
					Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "POST")

				if c.Request.Method == "OPTIONS" {
					c.AbortWithStatus(204)
					return
				}
			})

			// rate limit by ip
			r.Use(NewRateLimitMiddleware(func(gc *gin.Context) (string, error) {
				return gc.ClientIP(), nil
			}))

			// rate limit by address
			r.Use(NewRateLimitMiddleware(func(gc *gin.Context) (string, error) {
				var req Request
				if err := gc.ShouldBindBodyWith(&req, binding.JSON); err != nil {
					return "", err
				}
				return req.Address, nil
			}))

			r.POST("/request", func(gc *gin.Context) {
				handleRequest(gc, c)
			})

			return r.Run("0.0.0.0:" + cfg.Port)
		},
	}
	cmd.Flags().String(flags.FlagChainID, "", "chain ID of BandChain network")
	cmd.Flags().String(flags.FlagNode, "tcp://localhost:26657", "RPC url to BandChain node")
	cmd.Flags().String(flags.FlagGasPrices, "", "gas prices for report transaction")
	cmd.Flags().String(flagPort, "5005", "port of faucet service")
	cmd.Flags().Int64(flagAmount, 10000000, "amount in uband for each request")
	_ = viper.BindPFlag(flags.FlagChainID, cmd.Flags().Lookup(flags.FlagChainID))
	_ = viper.BindPFlag(flags.FlagNode, cmd.Flags().Lookup(flags.FlagNode))
	_ = viper.BindPFlag(flags.FlagGasPrices, cmd.Flags().Lookup(flags.FlagGasPrices))
	_ = viper.BindPFlag(flagPort, cmd.Flags().Lookup(flagPort))
	_ = viper.BindPFlag(flagAmount, cmd.Flags().Lookup(flagAmount))
	return cmd
}
