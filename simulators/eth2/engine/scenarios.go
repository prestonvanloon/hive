package main

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/rauljordan/engine-proxy/proxy"
)

var (
	VALIDATOR_COUNT           uint64 = 64
	SLOT_TIME                 uint64 = 3
	TERMINAL_TOTAL_DIFFICULTY        = big.NewInt(100)
)

func TransitionTestnet(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         1,
		MergeForkEpoch:          2,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: []node{
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestRPCError(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         1,
		MergeForkEpoch:          2,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: []node{
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	ctx := context.Background()
	t.Logf("INFO (%v): Waiting for Finality", t.TestID)
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: %v", err)
	}
	t.Logf("INFO (%v): Finality reached", t.TestID)
	t.Logf("INFO (%v): Verifying participation", t.TestID)
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("INFO (%v): Participation verified", t.TestID)
	t.Logf("INFO (%v): Verifying Execution Payload is Canonical", t.TestID)
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("INFO (%v): Execution Payload Verified to be Canonical", t.TestID)
	t.Logf("INFO (%v): Verifying Proposers", t.TestID)
	if err := testnet.VerifyProposers(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("INFO (%v): Proposers Verified", t.TestID)
	t.Logf("INFO (%v): Verifying EL Heads", t.TestID)
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("INFO (%v): EL Heads Verified", t.TestID)
	fields := make(map[string]interface{})
	fields["headBlockHash"] = "weird error"

	testnet.proxies[0].AddRequestCallback("engine_forkchoiceUpdatedV1", func(b []byte) *proxy.Spoof {
		t.Logf("Request callbacks: %s", b)
		return nil
	})
	testnet.proxies[0].AddResponseCallback("engine_forkchoiceUpdatedV1", func(b []byte) *proxy.Spoof {
		t.Logf("Response callbacks: %s", b)
		return nil
	})

	/*
		spoof := &proxy.Spoof{
			Method: "engine_forkchoiceUpdatedV1",
			Fields: fields,
		}
		testnet.proxies[0].AddRequest(spoof)
	*/

	time.Sleep(24 * time.Second)
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err == nil {
		t.Fatalf("Expected different heads after spoof %v", err)
	}
}
