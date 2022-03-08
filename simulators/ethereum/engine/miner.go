package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))

	errNoMiningWork      = errors.New("no mining work available yet")
	errInvalidSealResult = errors.New("invalid or stale proof-of-work solution")
)

func ethashDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ethash")
}

type HiveMiner struct {
	RPC    *rpc.Client
	Eth    *ethclient.Client
	Ethash *ethash.Ethash
	TTD    *big.Int

	// Context related
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

type SubmittableWork struct {
	Nonce         types.BlockNonce
	PoWHeaderHash common.Hash
	MixDigest     common.Hash
}

func (w SubmittableWork) String() string {
	return fmt.Sprintf("Nonce=%s, PoWHeaderHash=%s, MixDigest=%s", hexutil.Encode(w.Nonce[:]), w.PoWHeaderHash.Hex(), w.MixDigest.Hex())
}

type WorkDescription struct {
	PoWHeaderHash common.Hash
	SeedHash      common.Hash
	Target        *big.Int
	Difficulty    *big.Int
	BlockNumber   uint64
}

func (w WorkDescription) String() string {
	return fmt.Sprintf("PoWHeaderHash=%s, SeedHash=%s, Target=%v, Difficulty=%v, BlockNumber=%d",
		w.PoWHeaderHash.Hex(), w.SeedHash.Hex(), w.Target, w.Difficulty, w.BlockNumber)
}

func NewHiveMiner() *HiveMiner {
	config := ethash.Config{
		PowMode:        ethash.ModeNormal,
		CachesInMem:    2,
		DatasetsOnDisk: 2,
		DatasetDir:     ethashDir(),
	}

	etha := ethash.New(config, nil, false)
	etha.Dataset(0, true)

	return &HiveMiner{
		Ethash: etha,
	}
}

func (m *HiveMiner) UpdateEthEndpoint(rpcUrl string) error {
	client := &http.Client{}
	rpcClient, _ := rpc.DialHTTPWithClient(rpcUrl, client)
	eth := ethclient.NewClient(rpcClient)
	if m.RPC != nil {
		m.RPC.Close()
	}
	m.RPC = rpcClient
	if m.Eth != nil {
		m.Eth.Close()
	}
	m.Eth = eth
	return nil
}

func (m *HiveMiner) GetWork() (*WorkDescription, error) {
	var result [4]string
	if err := m.RPC.CallContext(m.Ctx(), &result, "eth_getWork"); err != nil {

		return nil, err
	}
	var (
		powHash     = common.HexToHash(result[0])
		seedHash    = common.HexToHash(result[1])
		target      = new(big.Int)
		blockNumber uint64
		err         error
	)

	target.SetBytes(common.HexToHash(result[2]).Bytes())

	if result[3] != "" {
		blockNumber, err = hexutil.DecodeUint64(result[3])
		if err != nil {
			return nil, err
		}
	} else {
		blockNumber, err = m.Eth.BlockNumber(m.Ctx())
		if err != nil {
			return nil, err
		}
		blockNumber++
	}
	return &WorkDescription{
		PoWHeaderHash: powHash,
		SeedHash:      seedHash,
		Target:        target,
		Difficulty:    new(big.Int).Div(two256, target),
		BlockNumber:   blockNumber,
	}, nil
}

func (m *HiveMiner) SubmitWork(subWork *SubmittableWork) (bool, error) {
	var result bool
	fmt.Printf("Sending eth_submitWork\n")
	if err := m.RPC.CallContext(m.Ctx(), &result, "eth_submitWork", hexutil.Encode(subWork.Nonce[:]), subWork.PoWHeaderHash.Hex(), subWork.MixDigest.Hex()); err != nil {
		return false, err
	}
	return result, nil
}

type TotalDifficulty struct {
	TotalDifficulty *hexutil.Big `json:"totalDifficulty"`
}

func (m *HiveMiner) NextBlockTD() (*big.Int, error) {
	w, err := m.GetWork()
	if err != nil {
		return nil, err
	}
	var td *TotalDifficulty
	if err := m.RPC.CallContext(m.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
		return nil, err
	}
	totalDifficulty := (*big.Int)(td.TotalDifficulty)

	return totalDifficulty.Add(totalDifficulty, w.Difficulty), nil
}

// Mine a block within a maximum time and a minimum time.
// If a block is mined before the minimumTime value has passed, the value is discarded and
// calculated again.
// The minimum time can be used to simulate an scenario where a difficulty bomb has gone off.
func (m *HiveMiner) MineBlock(timeout <-chan time.Time, minimumTime time.Duration) error {
	var (
		currentWork *WorkDescription
		err         error
	)
	for {
		currentWork, err = m.GetWork()
		if err != nil {
			if err.Error() != errNoMiningWork.Error() {
				return err
			}
			select {
			case <-time.After(time.Second):
			case <-timeout:
				return errors.New("Timeout while waiting for work")
			default:
			}
		} else {
			break
		}
	}
	abortChan := make(chan struct{})
	foundChan := make(chan *SubmittableWork)
	start := time.Now()
	go m.mine(currentWork, abortChan, foundChan)
loop:
	for {
		select {
		case <-timeout:
			close(abortChan)
			return errors.New("Timeout while waiting for block")
		case <-time.After(time.Second):
			newWork, err := m.GetWork()
			if err != nil {
				if err.Error() != errNoMiningWork.Error() {
					return err
				}
				break
			}
			if currentWork == nil || newWork.PoWHeaderHash != currentWork.PoWHeaderHash {
				if currentWork != nil {
					fmt.Printf("PoW Header Hash changed while mining: %v -> %v\n", currentWork.PoWHeaderHash, newWork.PoWHeaderHash)
				}
				close(abortChan)
				currentWork = newWork
				abortChan = make(chan struct{})
				fmt.Printf("Starting new miner\n")
				go m.mine(currentWork, abortChan, foundChan)
			}
		case subWork := <-foundChan:
			fmt.Printf("Found work after %v\n", time.Since(start))
			if time.Since(start) < minimumTime {
				// Start another miner because the minimum time has not passed yet
				fmt.Printf("Delaying block...\n")
				currentWork = nil
				break
			}
			fmt.Printf("Submiting work: %v\n", subWork)
			result, err := m.SubmitWork(subWork)
			if err != nil {
				return err
			}
			if result != true {
				return errors.New("Submitted work got rejected")
			}
			break loop
		}
	}

	return nil
}

// mine is the actual proof-of-work miner that searches for a nonce starting from
// seed that results in correct final block difficulty.
func (m *HiveMiner) mine(work *WorkDescription, abort <-chan struct{}, found chan *SubmittableWork) {
	// Extract some data from the header
	fmt.Printf("Generating Dataset (DAG)...\n")
	var (
		hash   = work.PoWHeaderHash.Bytes()
		target = work.Target
		number = work.BlockNumber
		d      = m.Ethash.Dataset(number, false)
	)
	// Start generating random nonces until we abort or find a good one
	var (
		totalAttempts = int64(0)
		attempts      = int64(0)
		nonce         = uint64(0)
		powBuffer     = new(big.Int)
	)
	m.Ethash.Threads()
search:
	for {
		select {
		case <-abort:
			// Mining terminated, update stats and abort
			break search

		default:
			// We don't have to update hash rate on every nonce, so update after after 2^X nonces
			attempts++
			if (attempts % (1 << 15)) == 0 {
				totalAttempts += attempts
				fmt.Printf("Total Attempts=%d, Nonce=%d\n", totalAttempts, nonce)
				attempts = 0
			}
			// Compute the PoW value of this nonce
			digest, result := ethash.HashimotoFull(d.Dataset, hash, nonce)
			if powBuffer.SetBytes(result).Cmp(target) <= 0 {

				subWork := &SubmittableWork{
					Nonce:         types.EncodeNonce(nonce),
					PoWHeaderHash: work.PoWHeaderHash,
					MixDigest:     common.BytesToHash(digest),
				}

				// Seal and return a block (if still needed)
				select {
				case found <- subWork:
				case <-abort:
				}
				break search
			}
			nonce++
		}
	}
	// Datasets are unmapped in a finalizer. Ensure that the dataset stays live
	// during sealing so it's not unmapped while being read.
	runtime.KeepAlive(d)
}

func (m *HiveMiner) Ctx() context.Context {
	if m.lastCtx != nil {
		m.lastCancel()
	}
	m.lastCtx, m.lastCancel = context.WithTimeout(context.Background(), 10*time.Second)
	return m.lastCtx
}
