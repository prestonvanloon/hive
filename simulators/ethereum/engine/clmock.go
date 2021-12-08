package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
)

// Consensus Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	EngineClients               []*EngineClient
	RandomHistory               map[uint64]common.Hash
	LastPoSChainHeader          *types.Header
	PoSBlockProductionActivated bool
	BlockProductionMustStop     bool
	FirstPoSBlockNumber         *big.Int
	CurrentBlockNumber          *big.Int
	TTDReached                  bool
	NextFeeRecipient            common.Address
	NextFeeRecipientMutex       sync.Mutex
	BlockProducedMutex          sync.Mutex
}

func NewCLMocker() *CLMocker {
	// Init random seed for different purposes
	rand.Seed(time.Now().Unix())
	// Create the new CL mocker
	newCLMocker := &CLMocker{
		EngineClients:               make([]*EngineClient, 0),
		RandomHistory:               map[uint64]common.Hash{},
		LastPoSChainHeader:          nil,
		PoSBlockProductionActivated: false,
		BlockProductionMustStop:     false,
		FirstPoSBlockNumber:         nil,
		CurrentBlockNumber:          nil,
		TTDReached:                  false,
		NextFeeRecipient:            common.Address{},
	}
	time.AfterFunc(tTDCheck, newCLMocker.checkTTD)
	newCLMocker.BlockProducedMutex.Lock()
	return newCLMocker
}

// Add a Client to be kept in sync with the latest payloads
func (cl *CLMocker) AddEngineClient(newEngineClient *EngineClient) {
	cl.EngineClients = append(cl.EngineClients, newEngineClient)
}

// Remove a Client to stop sending latest payloads
func (cl *CLMocker) RemoveCatalystClient(removeEngineClient *EngineClient) {
	i := -1
	for j := 0; j < len(cl.EngineClients); j++ {
		if cl.EngineClients[j] == removeEngineClient {
			i = j
			break
		}
	}
	if i >= 0 {
		cl.EngineClients[i] = cl.EngineClients[len(cl.EngineClients)-1]
		cl.EngineClients = cl.EngineClients[:len(cl.EngineClients)-1]
	}
}

// Checks whether we have reached TTD and enables PoS block production when done.
// This function must NOT be executed after we have reached TTD.
func (cl *CLMocker) checkTTD() {
	if len(cl.EngineClients) == 0 {
		// We have no clients running yet, we have not reached TTD
		time.AfterFunc(tTDCheck, cl.checkTTD)
		return
	}

	// Pick a random client to get the total difficulty of its head
	ec := cl.EngineClients[rand.Intn(len(cl.EngineClients))]

	lastBlockNumber, err := ec.Eth.BlockNumber(ec.Ctx())
	if err != nil {
		ec.Fatalf("Could not get block number: %v", err)
	}

	currentTD := big.NewInt(0)
	for i := 0; i <= int(lastBlockNumber); i++ {
		cl.LastPoSChainHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), big.NewInt(int64(i)))
		if err != nil {
			ec.Fatalf("Could not get block header: %v", err)
		}
		currentTD.Add(currentTD, cl.LastPoSChainHeader.Difficulty)
	}

	if currentTD.Cmp(terminalTotalDifficulty) >= 0 {
		cl.TTDReached = true
		fmt.Printf("TTD has been reached at block %v\n", cl.LastPoSChainHeader.Number)
		// Broadcast initial ForkchoiceUpdated
		forkchoiceState := catalyst.ForkchoiceStateV1{
			HeadBlockHash:      cl.LastPoSChainHeader.Hash(),
			SafeBlockHash:      cl.LastPoSChainHeader.Hash(),
			FinalizedBlockHash: cl.LastPoSChainHeader.Hash(),
		}
		for _, resp := range cl.broadcastForkchoiceUpdated(&forkchoiceState, nil) {
			if resp.Status != "SUCCESS" {
				fmt.Printf("forkchoiceUpdated Response: %v\n", resp)
			}
		}
		time.AfterFunc(blockProductionPoS, cl.minePOSBlock)
		return
	}
	time.AfterFunc(tTDCheck, cl.checkTTD)
}

// Engine Block Production Methods
func (cl *CLMocker) stopPoSBlockProduction() {
	cl.BlockProductionMustStop = true
}

func (cl *CLMocker) isBlockPoS(bn *big.Int) bool {
	if cl.FirstPoSBlockNumber == nil {
		return false
	}
	if cl.FirstPoSBlockNumber.Cmp(bn) <= 0 {
		return true
	}
	return false
}

// Sets the fee recipient for the next block and returns the number where it will be included.
func (cl *CLMocker) setNextFeeRecipient(feeRecipient common.Address) *big.Int {
	cl.NextFeeRecipientMutex.Lock()
	defer cl.NextFeeRecipientMutex.Unlock()
	cl.NextFeeRecipient = feeRecipient
	// Wait until the next block is produced using our feeRecipient
	cl.BlockProducedMutex.Lock()
	defer cl.BlockProducedMutex.Unlock()

	// Reset NextFeeRecipient
	cl.NextFeeRecipient = common.Address{}

	return cl.CurrentBlockNumber
}

// Mine a PoS block by using the catalyst Engine API
func (cl *CLMocker) minePOSBlock() {
	if cl.BlockProductionMustStop {
		cl.BlockProducedMutex.Unlock()
		return
	}
	var ec *EngineClient
	var lastBlockNumber uint64
	var err error
	for {
		// Get the client generating the payload
		ec_id := rand.Intn(len(cl.EngineClients))
		ec = cl.EngineClients[ec_id]

		lastBlockNumber, err = ec.Eth.BlockNumber(ec.Ctx())
		if err != nil {
			cl.BlockProducedMutex.Unlock()
			ec.Fatalf("Could not get block number: %v", err)
		}

		latestHeader, err := ec.Eth.HeaderByNumber(ec.Ctx(), big.NewInt(int64(lastBlockNumber)))
		if err != nil {
			cl.BlockProducedMutex.Unlock()
			ec.Fatalf("Could not get block header: %v", err)
		}

		lastBlockHash := latestHeader.Hash()

		if cl.LastPoSChainHeader.Hash() != lastBlockHash {
			continue
		} else {
			break
		}

	}

	forkchoiceState := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      cl.LastPoSChainHeader.Hash(),
		SafeBlockHash:      cl.LastPoSChainHeader.Hash(),
		FinalizedBlockHash: cl.LastPoSChainHeader.Hash(),
	}

	// Generate a random value for the Random field
	nextRandom := common.Hash{}
	for i := 0; i < common.HashLength; i++ {
		nextRandom[i] = byte(rand.Uint32())
	}

	payloadAttributes := catalyst.PayloadAttributesV1{
		Timestamp:             cl.LastPoSChainHeader.Time + 1,
		Random:                nextRandom,
		SuggestedFeeRecipient: cl.NextFeeRecipient,
	}

	// TODO: Maybe we can broadcast the forkchoiceUpdated to all clients in order to test that nothing breaks if we send
	// 		 another forkchoiceUpdated without getting the payload
	resp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), &forkchoiceState, &payloadAttributes)
	if err != nil {
		cl.BlockProducedMutex.Unlock()
		ec.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
	}
	if resp.Status != "SUCCESS" {
		fmt.Printf("forkchoiceUpdated Response: %v\n", resp)
	}

	payload, err := ec.EngineGetPayloadV1(ec.Ctx(), resp.PayloadID)
	if err != nil {
		cl.BlockProducedMutex.Unlock()
		ec.Fatalf("Could not getPayload (%v): %v", resp.PayloadID, err)
	}

	fmt.Printf("DEBUG: payload: %v\n", payload)

	// Broadcast the executed payload to all clients
	for _, resp := range cl.broadcastExecutePayload(&payload) {
		if resp.Status != "VALID" {
			fmt.Printf("resp: %v\n", resp)
		}
	}

	// Broadcast forkchoice updated to all clients
	forkchoiceState = catalyst.ForkchoiceStateV1{
		HeadBlockHash:      payload.BlockHash,
		SafeBlockHash:      payload.BlockHash,
		FinalizedBlockHash: payload.BlockHash,
	}
	for _, resp := range cl.broadcastForkchoiceUpdated(&forkchoiceState, nil) {
		if resp.Status != "SUCCESS" {
			fmt.Printf("resp: %v\n", resp)
		}
	}

	// Save random value
	cl.RandomHistory[cl.LastPoSChainHeader.Number.Uint64()+1] = nextRandom

	// Save the number of the first PoS block
	if cl.FirstPoSBlockNumber == nil {
		cl.FirstPoSBlockNumber = big.NewInt(int64(cl.LastPoSChainHeader.Number.Uint64() + 1))
	}

	// Save the header of the latest block in the PoS chain
	cl.CurrentBlockNumber = big.NewInt(int64(lastBlockNumber + 1))
	cl.LastPoSChainHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), cl.CurrentBlockNumber)
	if err != nil {
		cl.BlockProducedMutex.Unlock()
		ec.Fatalf("Could not get block header: %v", err)
	}

	// Trigger that we have finished producing a block
	cl.BlockProducedMutex.Unlock()

	// Exit if we need to
	if !cl.BlockProductionMustStop {
		// Lock BlockProducedMutex until we produce a new block
		cl.BlockProducedMutex.Lock()
		time.AfterFunc(blockProductionPoS, cl.minePOSBlock)
	}
}

func (cl *CLMocker) broadcastExecutePayload(payload *catalyst.ExecutableDataV1) []catalyst.ExecutePayloadResponse {
	responses := make([]catalyst.ExecutePayloadResponse, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		execPayloadResp, err := ec.EngineExecutePayloadV1(ec.Ctx(), payload)
		if err != nil {
			ec.Fatalf("Could not ExecutePayloadV1: %v", err)
		}
		responses[i] = execPayloadResp
	}
	return responses
}

func (cl *CLMocker) broadcastForkchoiceUpdated(fcstate *catalyst.ForkchoiceStateV1, payloadAttr *catalyst.PayloadAttributesV1) []catalyst.ForkChoiceResponse {
	responses := make([]catalyst.ForkChoiceResponse, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		fcUpdatedResp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), fcstate, payloadAttr)
		if err != nil {
			ec.Fatalf("Could not ForkchoiceUpdatedV1: %v", err)
		}
		responses[i] = fcUpdatedResp
	}
	return responses
}
