package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

func jsonStr(v interface{}) string {
	dat, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		panic(err)
	}
	return string(dat)
}

func main() {
	var suite = hivesim.Suite{
		Name:        "eth2-testnet",
		Description: `Run different eth2 testnets.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "testnets",
		Description: "Collection of different testnet compositions and assertions.",
		Run: func(t *hivesim.T) {
			clientTypes, err := t.Sim.ClientTypes()
			if err != nil {
				t.Fatal(err)
			}
			t.Log("clients by role:", jsonStr(clientTypes))
			byRole := ClientsByRole(clientTypes)
			t.Log("clients by role:", jsonStr(byRole))
			simpleTest := byRole.SimpleTestnetTest()
			t.Run(simpleTest)
			intermittentTest := byRole.IntermittentNetworkTestnetTest()
			t.Run(intermittentTest)
		},
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func (nc *ClientDefinitionsByRole) SimpleTestnetTest() hivesim.TestSpec {
	return hivesim.TestSpec{
		Name:        "single-client-testnet",
		Description: "This runs quick eth2 single-client type testnet, with 4 nodes and 2**14 (minimum) validators",
		Run: func(t *hivesim.T) {
			clientGroupCount := uint64(4)
			prep := prepareTestnet(t, 1<<8, clientGroupCount)
			testnet := prep.createTestnet(t)

			genesisTime := testnet.GenesisTime()
			countdown := genesisTime.Sub(time.Now())
			t.Logf("created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

			// TODO: we can mix things for a multi-client testnet
			if len(nc.Eth1) == 0 {
				t.Fatalf("choose at least 1 eth1 client type")
			}
			if len(nc.Beacon) == 0 {
				t.Fatalf("choose at least 1 beacon client type")
			}
			if len(nc.Validator) == 0 {
				t.Fatalf("choose at least 1 validator client type")
			}

			// for each key partition, we start a validator client with its own beacon node and eth1 node
			for i := 0; i < len(prep.keyTranches); i++ {
				prep.startEth1Node(testnet, nc.Eth1[i%len(nc.Eth1)], i)
				prep.startBeaconNode(testnet, nc.Beacon[i%len(nc.Beacon)], []int{i}, i)
				prep.startValidatorClient(testnet, nc.Validator[i%len(nc.Validator)], i, i, i)
			}
			t.Logf("started all nodes!")

			ctx := context.Background()

			// Add verification mechanism that signals testnet end when a certain epoch is reached
			// Epoch 3 is the final one
			endEpoch := common.Epoch(3)
			testnet.slotExecutableVerifications = append(testnet.slotExecutableVerifications, FinalEpochVerification(endEpoch))

			// Run until specified epoch is reached
			testnet.TrackFinality(ctx)
		},
	}
}

func (nc *ClientDefinitionsByRole) IntermittentNetworkTestnetTest() hivesim.TestSpec {
	return hivesim.TestSpec{
		Name:        "intermittent-single-client-testnet",
		Description: "This runs quick eth2 single-client type testnet, with 4 nodes and 2**14 (minimum) validators, beacon nodes are disconnected at a certain epoch",
		Run: func(t *hivesim.T) {
			clientGroupCount := uint64(4)
			prep := prepareTestnet(t, 1<<8, clientGroupCount)
			testnet := prep.createTestnet(t)

			genesisTime := testnet.GenesisTime()
			countdown := genesisTime.Sub(time.Now())
			t.Logf("created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

			// TODO: we can mix things for a multi-client testnet
			if len(nc.Eth1) == 0 {
				t.Fatalf("choose at least 1 eth1 client type")
			}
			if len(nc.Beacon) == 0 {
				t.Fatalf("choose at least 1 beacon client type")
			}
			if len(nc.Validator) == 0 {
				t.Fatalf("choose at least 1 validator client type")
			}

			// for each key partition, we start a validator client with its own beacon node and eth1 node
			for i := 0; i < len(prep.keyTranches); i++ {
				prep.startEth1Node(testnet, nc.Eth1[i%len(nc.Eth1)], i)
				prep.startBeaconNode(testnet, nc.Beacon[i%len(nc.Beacon)], []int{i}, i)
				prep.startValidatorClient(testnet, nc.Validator[i%len(nc.Validator)], i, i, i)
			}
			t.Logf("started all nodes!")

			ctx := context.Background()

			// Add verification mechanism that signals testnet end when a certain epoch is reached
			// Epoch 3 is the final one
			endEpoch := common.Epoch(3)
			testnet.slotExecutableVerifications = append(testnet.slotExecutableVerifications, FinalEpochVerification(endEpoch))

			// We are going to disconnect all clients from the Beaconchain network at slot 20 of each epoch,
			// and reconnect them at slot 23
			disconnectSlot := common.Slot(20)
			reconnectSlot := common.Slot(23)
			testnet.slotExecutableVerifications = append(testnet.slotExecutableVerifications, func(t *Testnet) {
				if len(t.beaconSlots) == 0 {
					// We haven't reached genesis
					return
				}
				for _, s := range t.beaconSlots {
					slotInEpoch := *s % t.spec.SLOTS_PER_EPOCH
					if slotInEpoch == disconnectSlot {
						t.T.Logf("Disconnect slot %v (%v) has been reached, disconnecting all clients from the Beaconchain network", disconnectSlot, *s)
						if err := t.networks[bcNetworkName].DisconnectAllClients(); err != nil {
							t.T.Log("Error disconnecting clients from the Beaconchain network", err)
						}
						return
					}
					if slotInEpoch == reconnectSlot {
						t.T.Logf("Reconnect slot %v (%v) has been reached, reconnecting all clients to the Beaconchain network", reconnectSlot, *s)
						if err := t.networks[bcNetworkName].ReconnectAllClients(); err != nil {
							t.T.Log("Error reconnecting clients to the Beaconchain network", err)
						}
						return
					}
				}

			})

			// Run until specified epoch is reached
			testnet.TrackFinality(ctx)
		},
	}
}

/*
	TODO More testnet ideas:

	Name:        "two-client-testnet",
	Description: "This runs quick eth2 testnets with combinations of 2 client types, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
	Name:        "all-client-testnet",
	Description: "This runs a quick eth2 testnet with all client types, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
	Name:        "cross-single-client-testnet",
	Description: "This runs a quick eth2 single-client testnet, but beacon nodes are matched with all validator types",
*/
