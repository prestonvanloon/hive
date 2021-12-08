package main

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (
	// parameters used for signing transactions
	chainID  = big.NewInt(7)
	gasPrice = big.NewInt(30 * params.GWei)

	// would be nice to use a networkID that's different from chainID,
	// but some clients don't support the distinction properly.
	networkID = big.NewInt(7)

	// PoS related
	terminalTotalDifficulty = big.NewInt(131072 + 25)
	blockProductionPoS      = time.Second * 1
	tTDCheck                = time.Second * 1
)

var clMocker *CLMocker
var vault *Vault

var clientEnv = hivesim.Params{
	"HIVE_NETWORK_ID":          networkID.String(),
	"HIVE_CHAIN_ID":            chainID.String(),
	"HIVE_FORK_HOMESTEAD":      "0",
	"HIVE_FORK_TANGERINE":      "0",
	"HIVE_FORK_SPURIOUS":       "0",
	"HIVE_FORK_BYZANTIUM":      "0",
	"HIVE_FORK_CONSTANTINOPLE": "0",
	"HIVE_FORK_PETERSBURG":     "0",
	"HIVE_FORK_ISTANBUL":       "0",
	"HIVE_FORK_MUIR_GLACIER":   "0",
	"HIVE_FORK_BERLIN":         "0",
	"HIVE_FORK_LONDON":         "0",
	// All tests use clique PoA to mine new blocks.
	"HIVE_CLIQUE_PERIOD":     "1",
	"HIVE_CLIQUE_PRIVATEKEY": "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c",
	"HIVE_MINER":             "658bdf435d810c91414ec09147daa6db62406379",
	// Merge related
	"HIVE_TERMINAL_TOTAL_DIFFICULTY": terminalTotalDifficulty.String(),
}

var files = map[string]string{
	"/genesis.json": "./init/genesis.json",
}

type testSpec struct {
	Name  string
	About string
	Run   func(*TestEnv)
}

var tests = []testSpec{

	// TerminalTotalDifficulty Genesis
	{Name: "http/GenesisBlockByHash", Run: genesisBlockByHashTest},
	{Name: "http/GenesisBlockByNumber", Run: genesisBlockByNumberTest},
	{Name: "http/GenesisHeaderByHash", Run: genesisHeaderByHashTest},
	{Name: "http/GenesisHeaderByNumber", Run: genesisHeaderByNumberTest},

	// Engine API
	{Name: "http/EngineAPINegative", Run: engineAPINegative},
	{Name: "http/FeeRecipient", Run: feeRecipient},
	// Random opcode tests
	{Name: "http/RandomOpcodeTx", Run: randomOpcodeTx},
}

func main() {
	suite := hivesim.Suite{
		Name: "engine",
		Description: `
Test Engine API.`[1:],
	}
	suite.Add(&hivesim.ClientTestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Parameters:  clientEnv,
		Files:       files,
		Run:         runSourceTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runSourceTest(t *hivesim.T, c *hivesim.Client) {
	clMocker = NewCLMocker()
	vault = newVault()

	ec := NewEngineClient(t, c)
	clMocker.AddEngineClient(ec)

	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal("can't get node peer-to-peer endpoint:", enode)
	}
	newParams := clientEnv.Set("HIVE_BOOTNODE", enode)

	t.RunAllClients(hivesim.ClientTestSpec{
		Name:        fmt.Sprintf("%s", c.Type),
		Description: "test",
		Parameters:  newParams,
		Files:       files,
		Run:         runAllTests,
	})
	clMocker.stopPoSBlockProduction()
}

// runAllTests runs the tests against a client instance.
// Most tests simply wait for tx inclusion in a block so we can run many tests concurrently.
func runAllTests(t *hivesim.T, c *hivesim.Client) {
	ec := NewEngineClient(t, c)
	clMocker.AddEngineClient(ec)
	parallelism := 40

	s := newSemaphore(parallelism)
	for _, test := range tests {
		test := test
		s.get()
		go func() {
			defer s.put()
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s (%s)", test.Name, c.Type),
				Description: test.About,
				Run: func(t *hivesim.T) {
					switch test.Name[:strings.IndexByte(test.Name, '/')] {
					case "http":
						runHTTP(t, c, ec, vault, clMocker, test.Run)
					case "ws":
						runWS(t, c, ec, vault, clMocker, test.Run)
					default:
						panic("bad test prefix in name " + test.Name)
					}
				},
			})
		}()
	}
	s.drain()
	clMocker.RemoveCatalystClient(ec)
}

type semaphore chan struct{}

func newSemaphore(n int) semaphore {
	s := make(semaphore, n)
	for i := 0; i < n; i++ {
		s <- struct{}{}
	}
	return s
}

func (s semaphore) get() { <-s }
func (s semaphore) put() { s <- struct{}{} }

func (s semaphore) drain() {
	for i := 0; i < cap(s); i++ {
		<-s
	}
}
