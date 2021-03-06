package keeper

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/althea-net/peggy/module/x/peggy/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatches(t *testing.T) {

	// SETUP
	// =====

	k, ctx, keepers := CreateTestEnv(t)
	var (
		mySender            = bytes.Repeat([]byte{1}, sdk.AddrLen)
		myReceiver          = types.NewEthereumAddress("eth receiver")
		myTokenContractAddr = types.NewEthereumAddress("my eth token address")
		myETHToken          = "myETHToken"
		voucherDenom        = types.NewVoucherDenom(myTokenContractAddr, myETHToken)
		now                 = time.Now().UTC()
	)
	// mint some voucher first
	allVouchers := sdk.Coins{sdk.NewInt64Coin(string(voucherDenom), 99999)}
	err := keepers.SupplyKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	keepers.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = keepers.BankKeeper.SetCoins(ctx, mySender, allVouchers)
	require.NoError(t, err)

	// store counterpart
	k.StoreCounterpartDenominator(ctx, myTokenContractAddr, myETHToken)

	denominator := types.NewBridgedDenominator(myTokenContractAddr, myETHToken)

	// CREATE FIRST BATCH
	// ==================

	// add some TX to the pool
	for i, v := range []int64{2, 3, 2, 1} {
		amount := sdk.NewInt64Coin(string(voucherDenom), int64(i+100))
		fee := sdk.NewInt64Coin(string(voucherDenom), v)
		_, err := k.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
		require.NoError(t, err)
	}
	// when
	ctx = ctx.WithBlockTime(now)

	// tx batch size is 2, so that some of them stay behind
	firstBatch, err := k.BuildOutgoingTXBatch(ctx, voucherDenom, 2)
	require.NoError(t, err)

	fmt.Printf("FIRST BATCH VALSET %T", firstBatch.Valset)

	// then batch is persisted
	gotFirstBatch := k.GetOutgoingTXBatch(ctx, firstBatch.TokenContract, firstBatch.Nonce)
	require.NotNil(t, gotFirstBatch)

	expFirstBatch := types.OutgoingTxBatch{
		Nonce: types.NewUInt64Nonce(1),
		Elements: []types.OutgoingTransferTx{
			{
				ID:          2,
				BridgeFee:   denominator.ToUint64ERC20Token(3),
				Sender:      mySender,
				DestAddress: myReceiver,
				Amount:      denominator.ToUint64ERC20Token(101),
			},
			{
				ID:          1,
				BridgeFee:   denominator.ToUint64ERC20Token(2),
				Sender:      mySender,
				DestAddress: myReceiver,
				Amount:      denominator.ToUint64ERC20Token(100),
			},
		},
		TotalFee:           denominator.ToUint64ERC20Token(5),
		BridgedDenominator: denominator,
		TokenContract:      types.EthereumAddress{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Valset:             types.Valset{Nonce: 0x12d687, Members: types.BridgeValidators(nil)},
	}
	assert.Equal(t, expFirstBatch, *gotFirstBatch)

	// and verify remaining available Tx in the pool
	var gotUnbatchedTx []types.OutgoingTx
	k.IterateOutgoingPoolByFee(ctx, voucherDenom, func(_ uint64, tx types.OutgoingTx) bool {
		gotUnbatchedTx = append(gotUnbatchedTx, tx)
		return false
	})
	expUnbatchedTx := []types.OutgoingTx{
		{
			BridgeFee:   sdk.NewInt64Coin(string(voucherDenom), 2),
			Sender:      mySender,
			DestAddress: myReceiver,
			Amount:      sdk.NewInt64Coin(string(voucherDenom), 102),
		},
		{
			BridgeFee:   sdk.NewInt64Coin(string(voucherDenom), 1),
			Sender:      mySender,
			DestAddress: myReceiver,
			Amount:      sdk.NewInt64Coin(string(voucherDenom), 103),
		},
	}
	assert.Equal(t, expUnbatchedTx, gotUnbatchedTx)

	// CREATE SECOND, MORE PROFITABLE BATCH
	// ====================================

	// add some more TX to the pool to create a more profitable batch
	for i, v := range []int64{4, 5} {
		amount := sdk.NewInt64Coin(string(voucherDenom), int64(i+100))
		fee := sdk.NewInt64Coin(string(voucherDenom), v)
		_, err := k.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
		require.NoError(t, err)
	}

	// create the more profitable batch
	ctx = ctx.WithBlockTime(now)
	// tx batch size is 2, so that some of them stay behind
	secondBatch, err := k.BuildOutgoingTXBatch(ctx, voucherDenom, 2)
	require.NoError(t, err)

	// check that the more profitable batch has the right txs in it
	expSecondBatch := types.OutgoingTxBatch{
		Nonce: types.NewUInt64Nonce(2),
		Elements: []types.OutgoingTransferTx{
			{
				ID:          6,
				BridgeFee:   denominator.ToUint64ERC20Token(5),
				Sender:      mySender,
				DestAddress: myReceiver,
				Amount:      denominator.ToUint64ERC20Token(101),
			},
			{
				ID:          5,
				BridgeFee:   denominator.ToUint64ERC20Token(4),
				Sender:      mySender,
				DestAddress: myReceiver,
				Amount:      denominator.ToUint64ERC20Token(100),
			},
		},
		TotalFee:           denominator.ToUint64ERC20Token(9),
		BridgedDenominator: denominator,
		TokenContract:      types.EthereumAddress{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		// For some reason, the empty Members field can be expressed by either []types.BridgeValidator{} or types.BridgeValidators(nil)
		Valset: types.Valset{Nonce: 0x12d687, Members: []types.BridgeValidator{}},
	}
	assert.Equal(t, expSecondBatch, *secondBatch)

	// EXECUTE THE MORE PROFITABLE BATCH
	// =================================

	// Execute the batch
	k.OutgoingTxBatchExecuted(ctx, secondBatch.TokenContract, secondBatch.Nonce)

	// check batch has been deleted
	gotSecondBatch := k.GetOutgoingTXBatch(ctx, secondBatch.TokenContract, secondBatch.Nonce)
	require.Nil(t, gotSecondBatch)

	// check that txs from first batch have been freed
	gotUnbatchedTx = nil
	k.IterateOutgoingPoolByFee(ctx, voucherDenom, func(_ uint64, tx types.OutgoingTx) bool {
		gotUnbatchedTx = append(gotUnbatchedTx, tx)
		return false
	})
	expUnbatchedTx = []types.OutgoingTx{
		{
			BridgeFee:   sdk.NewInt64Coin(string(voucherDenom), 3),
			Sender:      mySender,
			DestAddress: myReceiver,
			Amount:      sdk.NewInt64Coin(string(voucherDenom), 101),
		},
		{
			BridgeFee:   sdk.NewInt64Coin(string(voucherDenom), 2),
			Sender:      mySender,
			DestAddress: myReceiver,
			Amount:      sdk.NewInt64Coin(string(voucherDenom), 100),
		},
		{
			BridgeFee:   sdk.NewInt64Coin(string(voucherDenom), 2),
			Sender:      mySender,
			DestAddress: myReceiver,
			Amount:      sdk.NewInt64Coin(string(voucherDenom), 102),
		},
		{
			BridgeFee:   sdk.NewInt64Coin(string(voucherDenom), 1),
			Sender:      mySender,
			DestAddress: myReceiver,
			Amount:      sdk.NewInt64Coin(string(voucherDenom), 103),
		},
	}
	assert.Equal(t, expUnbatchedTx, gotUnbatchedTx)
}
