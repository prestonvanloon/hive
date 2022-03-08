package main

import (
	"time"
)

var bombTests = []TestSpec{

	{
		Name:           "PoW Reach to TTD",
		Run:            powReachToTTD,
		TTD:            4000000,
		RealPow:        true,
		TimeoutSeconds: 900,
	},
}

func powReachToTTD(t *TestEnv) {
	isNextBlockTTD := false
	minimumTime := time.Second * 13
	for {
		nextTD, err := t.HiveMiner.NextBlockTD()
		if err != nil {
			t.Fatalf("FAIL (%s): Error getting next Total Difficulty: %v", t.TestName, err)
		}
		t.Logf("INFO (%s): Next Total Difficulty = %v", t.TestName, nextTD)
		if nextTD.Cmp(t.MainTTD()) >= 0 {
			t.Logf("INFO (%s): Next block hits TTD of %v", t.TestName, t.MainTTD())
			isNextBlockTTD = true
			// Increase minimum time to simulate the difficulty bomb going off
			minimumTime = time.Minute
		}
		if err := t.HiveMiner.MineBlock(t.Timeout, minimumTime); err != nil {
			t.Fatalf("FAIL (%s): Error mining block: %v", t.TestName, err)
		}
		select {
		case <-t.Timeout:
			t.Fatalf("FAIL (%s): Timeout while mining TTD PoW block", t.TestName, err)
		default:
		}
		if isNextBlockTTD {
			break
		}
	}
	// We've hit TTD
	t.CLMock.waitForTTD()

	// Produce a couple of PoS blocks
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})
}
