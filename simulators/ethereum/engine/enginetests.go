package main

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/params"
)

var (
	contractCode = `
pragma solidity ^0.4.6;

contract Test {
    event E0();
    event E1(uint);
    event E2(uint indexed);
    event E3(address);
    event E4(address indexed);
    event E5(uint, address) anonymous;

    uint public ui;
    mapping(address => uint) map;

    function Test(uint ui_) {
    	ui = ui_;
        map[msg.sender] = ui_;
    }

    function events(uint ui_, address addr_) {
        E0();
        E1(ui_);
        E2(ui_);
        E3(addr_);
        E4(addr_);
        E5(ui_, addr_);
    }

    function constFunc(uint a, uint b, uint c) constant returns(uint, uint, uint) {
	    return (a, b, c);
    }

    function getFromMap(address addr) constant returns(uint) {
        return map[addr];
    }

    function addToMap(address addr, uint value) {
        map[addr] = value;
    }
}
	`
	// test contract deploy code, will deploy the contract with 1234 as argument
	deployCode = common.Hex2Bytes("6060604052346100005760405160208061048c833981016040528080519060200190919050505b8060008190555080600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b505b610409806100836000396000f30060606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a223e05d1461006a578063abd1a0cf1461008d578063abfced1d146100d4578063e05c914a14610110578063e6768b451461014c575b610000565b346100005761007761019d565b6040518082815260200191505060405180910390f35b34610000576100be600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506101a3565b6040518082815260200191505060405180910390f35b346100005761010e600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506101ed565b005b346100005761014a600480803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610236565b005b346100005761017960048080359060200190919080359060200190919080359060200190919050506103c4565b60405180848152602001838152602001828152602001935050505060405180910390f35b60005481565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b919050565b80600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b5050565b7f6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe85502260405180905060405180910390a17f47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450826040518082815260200191505060405180910390a1817fa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e2960405180905060405180910390a27f7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade81604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390a18073ffffffffffffffffffffffffffffffffffffffff167f7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e4160405180905060405180910390a28181604051808381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019250505060405180910390a05b5050565b6000600060008585859250925092505b935093509390505600a165627a7a72305820aaf842d0d0c35c45622c5263cbb54813d2974d3999c8c38551d7c613ea2bc117002900000000000000000000000000000000000000000000000000000000000004d2")
	// test contract code as deployed
	runtimeCode = common.Hex2Bytes("60606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a223e05d1461006a578063abd1a0cf1461008d578063abfced1d146100d4578063e05c914a14610110578063e6768b451461014c575b610000565b346100005761007761019d565b6040518082815260200191505060405180910390f35b34610000576100be600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506101a3565b6040518082815260200191505060405180910390f35b346100005761010e600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506101ed565b005b346100005761014a600480803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610236565b005b346100005761017960048080359060200190919080359060200190919080359060200190919050506103c4565b60405180848152602001838152602001828152602001935050505060405180910390f35b60005481565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b919050565b80600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b5050565b7f6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe85502260405180905060405180910390a17f47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450826040518082815260200191505060405180910390a1817fa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e2960405180905060405180910390a27f7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade81604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390a18073ffffffffffffffffffffffffffffffffffffffff167f7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e4160405180905060405180910390a28181604051808381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019250505060405180910390a05b5050565b6000600060008585859250925092505b935093509390505600a165627a7a72305820aaf842d0d0c35c45622c5263cbb54813d2974d3999c8c38551d7c613ea2bc1170029")
	// contractSrc is predeploy on the following address in the genesis block.
	predeployedContractAddr = common.HexToAddress("0000000000000000000000000000000000000314")
	// contractSrc is pre-deployed with the following address in the genesis block.
	predeployedContractWithAddress = common.HexToAddress("391694e7e0b0cce554cb130d723a9d27458f9298")
	// holds the pre-deployed contract ABI
	predeployedContractABI = `[{"constant":true,"inputs":[],"name":"ui","outputs":[{"name":"","type":"uint256"}],"payable":false,"type":"function"},{"constant":true,"inputs":[{"name":"addr","type":"address"}],"name":"getFromMap","outputs":[{"name":"","type":"uint256"}],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"addr","type":"address"},{"name":"value","type":"uint256"}],"name":"addToMap","outputs":[],"payable":false,"type":"function"},{"constant":false,"inputs":[{"name":"ui_","type":"uint256"},{"name":"addr_","type":"address"}],"name":"events","outputs":[],"payable":false,"type":"function"},{"constant":true,"inputs":[{"name":"a","type":"uint256"},{"name":"b","type":"uint256"},{"name":"c","type":"uint256"}],"name":"constFunc","outputs":[{"name":"","type":"uint256"},{"name":"","type":"uint256"},{"name":"","type":"uint256"}],"payable":false,"type":"function"},{"inputs":[{"name":"ui_","type":"uint256"}],"payable":false,"type":"constructor"},{"anonymous":false,"inputs":[],"name":"E0","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"name":"","type":"uint256"}],"name":"E1","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"name":"","type":"uint256"}],"name":"E2","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"name":"","type":"address"}],"name":"E3","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"name":"","type":"address"}],"name":"E4","type":"event"},{"anonymous":true,"inputs":[{"indexed":false,"name":"","type":"uint256"},{"indexed":false,"name":"","type":"address"}],"name":"E5","type":"event"}]`
)

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

// CodeAtTest tests the code for the pre-deployed contract.
func CodeAtTest(t *TestEnv) {
	code, err := t.Eth.CodeAt(t.Ctx(), predeployedContractAddr, big0)
	if err != nil {
		t.Fatalf("Could not fetch code for predeployed contract: %v", err)
	}
	if bytes.Compare(runtimeCode, code) != 0 {
		t.Fatalf("Unexpected code, want %x, got %x", runtimeCode, code)
	}
}

// estimateGasTest fetches the estimated gas usage for a call to the events method.
func estimateGasTest(t *TestEnv) {
	var (
		address        = t.Vault.createAccount(t, big.NewInt(params.Ether))
		contractABI, _ = abi.JSON(strings.NewReader(predeployedContractABI))
		intArg         = big.NewInt(rand.Int63())
	)

	payload, err := contractABI.Pack("events", intArg, address)
	if err != nil {
		t.Fatalf("Unable to prepare tx payload: %v", err)
	}
	msg := ethereum.CallMsg{
		From: address,
		To:   &predeployedContractAddr,
		Data: payload,
	}
	estimated, err := t.Eth.EstimateGas(t.Ctx(), msg)
	if err != nil {
		t.Fatalf("Could not estimate gas: %v", err)
	}

	// send the actual tx and test gas usage
	txGas := estimated + 100000
	rawTx := types.NewTransaction(0, *msg.To, msg.Value, txGas, big.NewInt(32*params.GWei), msg.Data)
	tx, err := t.Vault.signTransaction(address, rawTx)
	if err != nil {
		t.Fatalf("Could not sign transaction: %v", err)
	}

	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("Could not send tx: %v", err)
	}

	receipt, err := waitForTxConfirmations(t, tx.Hash(), 1)
	if err != nil {
		t.Fatalf("Could not wait for confirmations: %v", err)
	}

	// test lower bound
	if estimated < receipt.GasUsed {
		t.Fatalf("Estimated gas too low, want %d >= %d", estimated, receipt.GasUsed)
	}
	// test upper bound
	if receipt.GasUsed+5000 < estimated {
		t.Fatalf("Estimated gas too high, estimated: %d, used: %d", estimated, receipt.GasUsed)
	}
}

// balanceAndNonceAtTest creates a new account and transfers funds to it.
// It then tests if the balance and nonce of the sender and receiver
// address are updated correct.
func balanceAndNonceAtTest(t *TestEnv) {

	var (
		sourceAddr  = t.Vault.createAccount(t, big.NewInt(params.Ether))
		sourceNonce = uint64(0)
		targetAddr  = t.Vault.createAccount(t, nil)
	)

	// Get current balance
	sourceAddressBalanceBefore, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	expected := big.NewInt(params.Ether)
	if sourceAddressBalanceBefore.Cmp(expected) != 0 {
		t.Errorf("Expected balance %d, got %d", expected, sourceAddressBalanceBefore)
	}

	nonceBefore, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	if nonceBefore != sourceNonce {
		t.Fatalf("Invalid nonce, want %d, got %d", sourceNonce, nonceBefore)
	}

	// send 1234 wei to target account and verify balances and nonces are updated
	var (
		amount   = big.NewInt(1234)
		gasLimit = uint64(50000)
	)
	rawTx := types.NewTransaction(sourceNonce, targetAddr, amount, gasLimit, gasPrice, nil)
	valueTx, err := t.Vault.signTransaction(sourceAddr, rawTx)
	if err != nil {
		t.Fatalf("Unable to sign value tx: %v", err)
	}
	sourceNonce++

	t.Logf("BalanceAt: send %d wei from 0x%x to 0x%x in 0x%x", valueTx.Value(), sourceAddr, targetAddr, valueTx.Hash())
	if err := t.Eth.SendTransaction(t.Ctx(), valueTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	var receipt *types.Receipt
	for {
		receipt, err = t.Eth.TransactionReceipt(t.Ctx(), valueTx.Hash())
		if receipt != nil {
			break
		}
		if err != ethereum.NotFound {
			t.Fatalf("Could not fetch receipt for 0x%x: %v", valueTx.Hash(), err)
		}
		time.Sleep(time.Second)
	}

	// ensure balances have been updated
	accountBalanceAfter, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}
	balanceTargetAccountAfter, err := t.Eth.BalanceAt(t.Ctx(), targetAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	// expected balance is previous balance - tx amount - tx fee (gasUsed * gasPrice)
	exp := new(big.Int).Set(sourceAddressBalanceBefore)
	exp.Sub(exp, amount)
	exp.Sub(exp, new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), valueTx.GasPrice()))

	if exp.Cmp(accountBalanceAfter) != 0 {
		t.Errorf("Expected sender account to have a balance of %d, got %d", exp, accountBalanceAfter)
	}
	if balanceTargetAccountAfter.Cmp(amount) != 0 {
		t.Errorf("Expected new account to have a balance of %d, got %d", valueTx.Value(), balanceTargetAccountAfter)
	}

	// ensure nonce is incremented by 1
	nonceAfter, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	expectedNonce := nonceBefore + 1
	if expectedNonce != nonceAfter {
		t.Fatalf("Invalid nonce, want %d, got %d", expectedNonce, nonceAfter)
	}

}

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
	// Wait until TTD is reached
	for {
		if clMocker.TTDReached {
			bn, err := t.Eth.BlockNumber(t.Ctx())
			if err != nil {
				t.Fatalf("Unable to get block number: %v", err)
			}
			if clMocker.FirstPoSBlockNumber != nil && bn >= clMocker.FirstPoSBlockNumber.Uint64() {
				break
			}
		}
		time.Sleep(time.Second)
	}
	// We have reached TTD and the client is synced past the TTD block

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
