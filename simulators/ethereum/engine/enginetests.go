package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/params"
)

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

// genesisByHash fetches the known genesis header and compares
// it against the genesis file to determine if block fields are
// returned correct.
func genesisHeaderByHashTest(t *TestEnv) {
	gblock := loadGenesis()

	headerByHash, err := t.Eth.HeaderByHash(t.Ctx(), gblock.Hash())
	if err != nil {
		t.Fatalf("Unable to fetch block %x: %v", gblock.Hash(), err)
	}
	if d := diff(gblock.Header(), headerByHash); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// headerByNumberTest fetched the known genesis header and compares
// it against the genesis file to determine if block fields are
// returned correct.
func genesisHeaderByNumberTest(t *TestEnv) {
	gblock := loadGenesis()

	headerByNum, err := t.Eth.HeaderByNumber(t.Ctx(), big0)
	if err != nil {
		t.Fatalf("Unable to fetch genesis block: %v", err)
	}
	if d := diff(gblock.Header(), headerByNum); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// genesisBlockByHashTest fetched the known genesis block and compares it against
// the genesis file to determine if block fields are returned correct.
func genesisBlockByHashTest(t *TestEnv) {
	gblock := loadGenesis()

	blockByHash, err := t.Eth.BlockByHash(t.Ctx(), gblock.Hash())
	if err != nil {
		t.Fatalf("Unable to fetch block %x: %v", gblock.Hash(), err)
	}
	if d := diff(gblock.Header(), blockByHash.Header()); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// genesisBlockByNumberTest retrieves block 0 since that is the only block
// that is known through the genesis.json file and tests if block
// fields matches the fields defined in the genesis file.
func genesisBlockByNumberTest(t *TestEnv) {
	gblock := loadGenesis()

	blockByNum, err := t.Eth.BlockByNumber(t.Ctx(), big0)
	if err != nil {
		t.Fatalf("Unable to fetch genesis block: %v", err)
	}
	if d := diff(gblock.Header(), blockByNum.Header()); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// canonicalChainTest loops over 10 blocks and does some basic validations
// to ensure the chain form a valid canonical chain and resources like uncles,
// transactions and receipts can be fetched and provide a consistent view.
func canonicalChainTest(t *TestEnv) {
	// wait a bit so there is actually a chain with enough height
	for {
		latestBlock, err := t.Eth.BlockByNumber(t.Ctx(), nil)
		if err != nil {
			t.Fatalf("Unable to fetch latest block")
		}
		if latestBlock.NumberU64() >= 20 {
			break
		}
		time.Sleep(time.Second)
	}

	var childBlock *types.Block
	for i := 10; i >= 0; i-- {
		block, err := t.Eth.BlockByNumber(t.Ctx(), big.NewInt(int64(i)))
		if err != nil {
			t.Fatalf("Unable to fetch block #%d", i)
		}
		if childBlock != nil {
			if childBlock.ParentHash() != block.Hash() {
				t.Errorf("Canonical chain broken on %d-%d / %x-%x", block.NumberU64(), childBlock.NumberU64(), block.Hash(), childBlock.Hash())
			}
		}

		// try to fetch all txs and receipts and do some basic validation on them
		// to check if the fetched chain is consistent.
		for _, tx := range block.Transactions() {
			fetchedTx, _, err := t.Eth.TransactionByHash(t.Ctx(), tx.Hash())
			if err != nil {
				t.Fatalf("Unable to fetch transaction %x from block %x: %v", tx.Hash(), block.Hash(), err)
			}
			if fetchedTx == nil {
				t.Fatalf("Transaction %x could not be found but was included in block %x", tx.Hash(), block.Hash())
			}
			receipt, err := t.Eth.TransactionReceipt(t.Ctx(), fetchedTx.Hash())
			if err != nil {
				t.Fatalf("Unable to fetch receipt for %x from block %x: %v", fetchedTx.Hash(), block.Hash(), err)
			}
			if receipt == nil {
				t.Fatalf("Receipt for %x could not be found but was included in block %x", fetchedTx.Hash(), block.Hash())
			}
			if receipt.TxHash != fetchedTx.Hash() {
				t.Fatalf("Receipt has an invalid tx, expected %x, got %x", fetchedTx.Hash(), receipt.TxHash)
			}
		}

		// make sure all uncles can be fetched
		for _, uncle := range block.Uncles() {
			uBlock, err := t.Eth.HeaderByHash(t.Ctx(), uncle.Hash())
			if err != nil {
				t.Fatalf("Unable to fetch uncle block: %v", err)
			}
			if uBlock == nil {
				t.Logf("Could not fetch uncle block %x", uncle.Hash())
			}
		}

		childBlock = block
	}
}

// Engine API Tests
func engineAPINegative(t *TestEnv) {
	// Test that the engine_ directives are correctly ignored when the chain has not yet reached TTD
	gblock := loadGenesis()
	if !clMocker.TTDReached {
		// We can only execute this test if we have not yet reached the TTD.
		forkchoiceState := catalyst.ForkchoiceStateV1{
			HeadBlockHash:      gblock.Hash(),
			SafeBlockHash:      gblock.Hash(),
			FinalizedBlockHash: gblock.Hash(),
		}
		resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceState, nil)
		if err == nil {
			t.Fatalf("ForkchoiceUpdated accepted under PoW rule: %v, %v", resp, err)
		}
		payloadresp, err := t.Engine.EngineGetPayloadV1(t.Engine.Ctx(), &hexutil.Bytes{1, 2, 3, 4, 5, 6, 7, 8})
		if err == nil {
			t.Fatalf("GetPayloadV1 accepted under PoW rule: %v, %v", payloadresp, err)
		}
		// Create a dummy payload to send in the ExecutePayload call
		payload := catalyst.ExecutableDataV1{
			ParentHash:    common.Hash{},
			FeeRecipient:  common.Address{},
			StateRoot:     common.Hash{},
			ReceiptsRoot:  common.Hash{},
			LogsBloom:     []byte{},
			Random:        common.Hash{},
			Number:        0,
			GasLimit:      0,
			GasUsed:       0,
			Timestamp:     0,
			ExtraData:     []byte{},
			BaseFeePerGas: big.NewInt(0),
			BlockHash:     common.Hash{},
			Transactions:  [][]byte{},
		}
		execresp, err := t.Engine.EngineExecutePayloadV1(t.Engine.Ctx(), &payload)
		if err == nil {
			t.Fatalf("ExecutePayloadV1 accepted under PoW rule: %v, %v", execresp, err)
		}
	}

	// Wait until TTD is reached by this client
	waitForPoSSync(t)

	// We have reached TTD and the client is synced past the TTD block
	{
		// TODO 1: Add test where safeBlockHash is NOT equal to headBlockHash nor an ancestor to it.
		// TODO 1b: Same but for finalizedBlockHash

		/* TODO 2: Enable
		// Send a forkchoiceUpdated with unknown random Block hash
		randomHeadBlockHash := common.Hash{}
		for i := 0; i < common.HashLength; i++ {
			randomHeadBlockHash[i] = byte(rand.Uint32())
		}
		forkchoiceStateUnknownHash := catalyst.ForkchoiceStateV1{
			HeadBlockHash:      randomHeadBlockHash,
			SafeBlockHash:      randomHeadBlockHash,
			FinalizedBlockHash: randomHeadBlockHash,
		}

		resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownHash, nil)
		if err != nil {
			t.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
		}
		if resp.Status != "SYNCING" {
			t.Fatalf("Unknown hash forkchoiceUpdatedV1 did not return SYNCING: %v", resp)
		}
		*/
	}
}

// Test to get information on the latest HeadBlock
func blockStatus(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	waitForPoSSync(t)

	// Run HeadBlock tests
	clMocker.BlockHeadMutex.Lock()
	currentBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		clMocker.BlockHeadMutex.Unlock()
		t.Fatalf("BlockStatus: Unable to get latest block header: %v", err)
	}
	/* TODO: Geth changes immediately when HeadBlockHash is received
	if currentBlockHeader.Hash() == clMocker.LatestForkchoice.HeadBlockHash ||
		currentBlockHeader.Hash() != clMocker.LatestForkchoice.SafeBlockHash ||
		currentBlockHeader.Hash() != clMocker.LatestForkchoice.FinalizedBlockHash {
		clMocker.BlockHeadMutex.Unlock()
		t.Fatalf("BlockStatus: latest block header doesn't match SafeBlock hash: %v, %v", currentBlockHeader.Hash(), clMocker.LatestForkchoice)
	}
	*/
	clMocker.BlockHeadMutex.Unlock()

	// Run SafeBlock tests
	clMocker.BlockSafeMutex.Lock()
	currentBlockHeader, err = t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatus: Unable to get latest block header: %v", err)
	}
	if currentBlockHeader.Hash() != clMocker.LatestForkchoice.HeadBlockHash ||
		currentBlockHeader.Hash() != clMocker.LatestForkchoice.SafeBlockHash ||
		currentBlockHeader.Hash() == clMocker.LatestForkchoice.FinalizedBlockHash {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatus: latest block header doesn't match SafeBlock hash: %v, %v", currentBlockHeader.Hash(), clMocker.LatestForkchoice)
	}
	clMocker.BlockSafeMutex.Unlock()

	// Run FinalizedBlock tests
	clMocker.BlockFinalMutex.Lock()
	currentBlockHeader, err = t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		clMocker.BlockFinalMutex.Unlock()
		t.Fatalf("BlockStatus: Unable to get latest block header: %v", err)
	}
	if currentBlockHeader.Hash() != clMocker.LatestForkchoice.HeadBlockHash ||
		currentBlockHeader.Hash() != clMocker.LatestForkchoice.SafeBlockHash ||
		currentBlockHeader.Hash() != clMocker.LatestForkchoice.FinalizedBlockHash {
		clMocker.BlockFinalMutex.Unlock()
		t.Fatalf("BlockStatus: latest block header doesn't match FinalizedBlock hash: %v, %v", currentBlockHeader.Hash(), clMocker.LatestForkchoice)
	}
	clMocker.BlockFinalMutex.Unlock()
}

// Test to get information on the latest HeadBlock
func blockStatusReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	waitForPoSSync(t)

	// Wait until we reach SafeBlock status
	clMocker.BlockSafeMutex.Lock()

	// Verify the client is serving the latest SafeBlock
	currentBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: Unable to get latest block header: %v", err)
	}
	if currentBlockHeader.Hash() != clMocker.LatestForkchoice.HeadBlockHash ||
		currentBlockHeader.Hash() != clMocker.LatestForkchoice.SafeBlockHash ||
		currentBlockHeader.Hash() == clMocker.LatestForkchoice.FinalizedBlockHash {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: latest block header doesn't match SafeBlock hash: %v, %v", currentBlockHeader.Hash(), clMocker.LatestForkchoice)
	}

	// Reorg back to the previous block (FinalizedBlock)
	reorgForkchoice := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      clMocker.LatestForkchoice.FinalizedBlockHash,
		SafeBlockHash:      clMocker.LatestForkchoice.FinalizedBlockHash,
		FinalizedBlockHash: clMocker.LatestForkchoice.FinalizedBlockHash,
	}
	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &reorgForkchoice, nil)
	if err != nil {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: Could not send forkchoiceUpdatedV1: %v", err)
	}
	if resp.Status != "SUCCESS" {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: Could not send forkchoiceUpdatedV1: %v", err)
	}

	// Check that we reorg to the previous block
	currentBlockHeader, err = t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: Unable to get latest block header: %v", err)
	}

	if currentBlockHeader.Hash() != clMocker.LatestForkchoice.FinalizedBlockHash {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: latest block header doesn't match reorg hash: %v, %v", currentBlockHeader.Hash(), clMocker.LatestForkchoice)
	}

	// Send the SafeBlock again to leave everything back the way it was
	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &clMocker.LatestForkchoice, nil)
	if err != nil {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: Could not send forkchoiceUpdatedV1: %v", err)
	}
	if resp.Status != "SUCCESS" {
		clMocker.BlockSafeMutex.Unlock()
		t.Fatalf("BlockStatusReorg: Could not send forkchoiceUpdatedV1: %v", err)
	}

	clMocker.BlockSafeMutex.Unlock()
}

// Test to re-org to a previous hash
func transactionReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	waitForPoSSync(t)

	// Wait for the latest HeadBlock forkchoice to be broadcast
	clMocker.BlockHeadMutex.Lock()
	// Run HeadBlock tests
	clMocker.BlockHeadMutex.Unlock()
	clMocker.BlockSafeMutex.Lock()
	// Run SafeBlock tests
	clMocker.BlockSafeMutex.Unlock()
	clMocker.BlockFinalMutex.Lock()
	// Run FinalizedBlock tests
	clMocker.BlockFinalMutex.Unlock()
}

// Fee Recipient Tests
func feeRecipient(t *TestEnv) {
	timeout := 60
	for i := 0; i <= timeout; i++ {
		if clMocker.TTDReached {
			break
		}
		if i == timeout {
			t.Fatalf("FeeRecipient: TTD was never reached")
		}
		time.Sleep(time.Second)
	}
	for i := 1; i <= 10; i++ {
		feeRecipientAddress := common.Address{byte(i)}
		blockNumberIncluded := clMocker.setNextFeeRecipient(feeRecipientAddress)
		if blockNumberIncluded == nil {
			t.Fatalf("FeeRecipient: unable to get block number included")
		}
		blockIncluded, err := waitForBlock(t, blockNumberIncluded)
		if err != nil {
			t.Fatalf("FeeRecipient: unable to get block with fee recipient: %v", err)
		}
		if blockIncluded.Coinbase() != feeRecipientAddress {
			t.Fatalf("FeeRecipient: Expected feeRecipient is not header.coinbase: %v!=%v", blockIncluded.Coinbase, feeRecipientAddress)
		}
		balanceAfter, err := t.Eth.BalanceAt(t.Ctx(), feeRecipientAddress, blockNumberIncluded)
		if err != nil {
			t.Fatalf("FeeRecipient: Unable to obtain balanceAfter: %v", err)
		}
		balanceBefore, err := t.Eth.BalanceAt(t.Ctx(), feeRecipientAddress, blockNumberIncluded.Sub(blockNumberIncluded, big.NewInt(1)))
		if err != nil {
			t.Fatalf("FeeRecipient: Unable to obtain balanceBefore: %v", err)
		}
		balDiff := big.NewInt(0).Sub(balanceAfter, balanceBefore)

		feeRecipientFee := big.NewInt(0)
		for _, tx := range blockIncluded.Transactions() {
			effGasTip, err := tx.EffectiveGasTip(blockIncluded.BaseFee())
			if err != nil {
				t.Fatalf("FeeRecipient: unable to obtain EffectiveGasTip: %v", err)
			}
			receipt, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
			if err != nil {
				t.Fatalf("FeeRecipient: unable to obtain receipt: %v", err)
			}
			feeRecipientFee = feeRecipientFee.Add(feeRecipientFee, effGasTip.Mul(effGasTip, big.NewInt(int64(receipt.GasUsed))))
		}

		if feeRecipientFee.Cmp(balDiff) != 0 {
			t.Fatalf("FeeRecipient: actual fee received does not match feeRecipientFee: %v, %v", balDiff, feeRecipientFee)
		}
	}

}

// Random Opcode tests
func randomOpcodeTx(t *TestEnv) {
	var (
		key                = t.Vault.createAccount(t, big.NewInt(params.Ether))
		nonce              = uint64(0)
		txCount            = 20
		randomContractAddr = common.HexToAddress("0000000000000000000000000000000000000316")
	)
	var txs = make([]*types.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		rawTx := types.NewTransaction(nonce, randomContractAddr, big0, 100000, gasPrice, nil)
		nonce++
		tx, err := t.Vault.signTransaction(key, rawTx)
		txs[i] = tx
		if err != nil {
			t.Fatalf("Random: Unable to sign deploy tx: %v", err)
		}
		if err = t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
			bal, _ := t.Eth.BalanceAt(t.Ctx(), key, nil)
			fmt.Printf("Random: balance=%v\n", bal)
			t.Fatalf("Random: Unable to send transaction: %v", err)
		}
		time.Sleep(blockProductionPoS / 2)
	}
	PoWBlocks := 0
	PoSBlocks := 0
	i := 0
	for {
		receipt, err := waitForTxConfirmations(t, txs[i].Hash(), 15)
		if err != nil {
			t.Errorf("Random: Unable to fetch confirmed tx receipt: %v", err)
		}
		if receipt == nil {
			t.Errorf("Random: Unable to confirm tx: %v", txs[i].Hash())
		}

		blockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), receipt.BlockNumber)
		if err != nil {
			t.Errorf("Random: Unable to fetch block header: %v", err)
		}

		stor, err := t.Eth.StorageAt(t.Ctx(), randomContractAddr, common.BigToHash(receipt.BlockNumber), receipt.BlockNumber)
		if err != nil {
			t.Errorf("Random: Unable to fetch storage: %v, block hash=%v", err, receipt.BlockHash)
		}
		storHash := common.BytesToHash(stor)

		if clMocker.isBlockPoS(receipt.BlockNumber) {
			PoSBlocks++
			if clMocker.RandomHistory[receipt.BlockNumber.Uint64()] != storHash {
				t.Fatalf("Random: Storage does not match random: %v, %v", clMocker.RandomHistory[receipt.BlockNumber.Uint64()], storHash)
			}
			if blockHeader.Difficulty.Cmp(big.NewInt(0)) != 0 {
				t.Fatalf("Random: PoS Block (%v) difficulty not set to zero: %v", receipt.BlockNumber, blockHeader.Difficulty)
			}
			if blockHeader.MixDigest != storHash {
				t.Fatalf("Random: PoS Block (%v) mix digest does not match random: %v", blockHeader.MixDigest, storHash)
			}
		} else {
			PoWBlocks++
			if blockHeader.Difficulty.Cmp(storHash.Big()) != 0 {
				t.Fatalf("Random: Storage does not match difficulty: %v, %v", blockHeader.Difficulty, storHash)
			}
			if blockHeader.Difficulty.Cmp(big.NewInt(0)) == 0 {
				t.Fatalf("Random: PoW Block (%v) difficulty is set to zero: %v", receipt.BlockNumber, blockHeader.Difficulty)
			}
		}

		i++
		if i >= txCount {
			break
		}
	}
	if PoSBlocks == 0 {
		t.Fatalf("Random: No Random Opcode transactions landed in PoS blocks")
	}
}
