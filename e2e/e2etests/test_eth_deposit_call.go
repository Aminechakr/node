package e2etests

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/zeta-chain/zetacore/e2e/runner"
	"github.com/zeta-chain/zetacore/e2e/utils"
	testcontract "github.com/zeta-chain/zetacore/testutil/contracts"
	"github.com/zeta-chain/zetacore/x/crosschain/types"
)

// TestEtherDepositAndCall tests deposit of ethers calling a example contract
func TestEtherDepositAndCall(r *runner.E2ERunner, args []string) {
	require.Len(r, args, 1)

	value, ok := big.NewInt(0).SetString(args[0], 10)
	require.True(r, ok, "Invalid amount specified for TestEtherDepositAndCall.")

	r.Logger.Info("Deploying example contract")
	exampleAddr, _, exampleContract, err := testcontract.DeployExample(r.ZEVMAuth, r.ZEVMClient)
	require.NoError(r, err)

	r.Logger.Info("Example contract deployed")

	// preparing tx
	evmClient := r.EVMClient
	gasLimit := uint64(23000)
	gasPrice, err := evmClient.SuggestGasPrice(r.Ctx)
	require.NoError(r, err)

	nonce, err := evmClient.PendingNonceAt(r.Ctx, r.EVMAddress())
	require.NoError(r, err)

	data := append(exampleAddr.Bytes(), []byte("hello sailors")...)
	tx := ethtypes.NewTransaction(nonce, r.TSSAddress, value, gasLimit, gasPrice, data)
	chainID, err := evmClient.NetworkID(r.Ctx)
	require.NoError(r, err)

	deployerPrivkey, err := r.Account.PrivateKey()
	require.NoError(r, err)

	signedTx, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), deployerPrivkey)
	require.NoError(r, err)

	r.Logger.Info("Sending a cross-chain call to example contract")
	err = evmClient.SendTransaction(r.Ctx, signedTx)
	require.NoError(r, err)

	receipt := utils.MustWaitForTxReceipt(r.Ctx, r.EVMClient, signedTx, r.Logger, r.ReceiptTimeout)
	utils.RequireTxSuccessful(r, receipt)

	cctx := utils.WaitCctxMinedByInboundHash(r.Ctx, signedTx.Hash().Hex(), r.CctxClient, r.Logger, r.CctxTimeout)
	utils.RequireCCTXStatus(r, cctx, types.CctxStatus_OutboundMined)

	// Checking example contract has been called, bar value should be set to amount
	bar, err := exampleContract.Bar(&bind.CallOpts{})
	require.NoError(r, err)
	require.Equal(
		r,
		0,
		bar.Cmp(value),
		"cross-chain call failed bar value %s should be equal to amount %s",
		bar.String(),
		value.String(),
	)
	r.Logger.Info("Cross-chain call succeeded")

	r.Logger.Info("Deploying reverter contract")
	reverterAddr, _, _, err := testcontract.DeployReverter(r.ZEVMAuth, r.ZEVMClient)
	require.NoError(r, err)

	r.Logger.Info("Example reverter deployed")

	// preparing tx for reverter
	gasPrice, err = evmClient.SuggestGasPrice(r.Ctx)
	require.NoError(r, err)

	nonce, err = evmClient.PendingNonceAt(r.Ctx, r.EVMAddress())
	require.NoError(r, err)

	data = append(reverterAddr.Bytes(), []byte("hello sailors")...)
	tx = ethtypes.NewTransaction(nonce, r.TSSAddress, value, gasLimit, gasPrice, data)
	signedTx, err = ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), deployerPrivkey)
	require.NoError(r, err)

	r.Logger.Info("Sending a cross-chain call to reverter contract")
	err = evmClient.SendTransaction(r.Ctx, signedTx)
	require.NoError(r, err)

	receipt = utils.MustWaitForTxReceipt(r.Ctx, r.EVMClient, signedTx, r.Logger, r.ReceiptTimeout)
	utils.RequireTxSuccessful(r, receipt)

	cctx = utils.WaitCctxMinedByInboundHash(r.Ctx, signedTx.Hash().Hex(), r.CctxClient, r.Logger, r.CctxTimeout)
	utils.RequireCCTXStatus(r, cctx, types.CctxStatus_Reverted)

	r.Logger.Info("Cross-chain call to reverter reverted")

	// check the status message contains revert error hash in case of revert
	// 0xbfb4ebcf is the hash of "Foo()"
	require.Contains(r, cctx.CctxStatus.StatusMessage, "0xbfb4ebcf")
}
