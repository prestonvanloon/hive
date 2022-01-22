package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

var (
	elNetworkName = "ExecutionNetwork"
	bcNetworkName = "BeaconChainNetwork"
)

type Testnet struct {
	T *hivesim.T

	genesisTime           common.Timestamp
	genesisValidatorsRoot common.Root

	// Consensus chain configuration
	spec *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis *setup.Eth1Genesis

	// Clients lists
	beacons    []*BeaconNode
	validators []*ValidatorClient
	eth1       []*Eth1Node

	// Client Networks
	networks map[string]*ClientNetwork

	// Beacon Slots
	beaconSlots []*common.Slot

	// Slot executables
	slotExecutableVerifications []func(*Testnet)

	// Group Networks Names (Group = EL + BC + VN Clients)
	groupNetworks map[int]string

	testnetMustFinishSignal bool
}

func (t *Testnet) CreateNetworks() error {
	var err error
	// Create Execution Layer network
	if t.networks[elNetworkName], err = NewClientNetwork(t.T, elNetworkName); err != nil {
		return err
	}
	// Create Beacon Chain network
	if t.networks[bcNetworkName], err = NewClientNetwork(t.T, bcNetworkName); err != nil {
		return err
	}
	return nil
}

func (t *Testnet) RemoveNetworks() error {
	for n := range t.networks {
		if err := t.networks[n].RemoveNetwork(); err != nil {
			return err
		}
	}
	return nil
}

func (t *Testnet) ConnectClientToNetwork(c *hivesim.Client, networkName string) error {
	network, found := t.networks[networkName]
	if !found {
		return errors.New("network not found")
	}
	return network.ConnectClient(c.Container)
}

func (t *Testnet) AddClientToGroupNetwork(c *hivesim.Client, groupNetworkIndex int) error {
	var err error
	networkName, found := t.groupNetworks[groupNetworkIndex]
	if !found {
		// This is the first client added to the group, we need to create the network
		networkName = fmt.Sprintf("GroupNetwork%03d", groupNetworkIndex)
		if t.networks[networkName], err = NewClientNetwork(t.T, networkName); err != nil {
			return err
		}
		t.groupNetworks[groupNetworkIndex] = networkName
	}
	return t.ConnectClientToNetwork(c, networkName)
}

func (t *Testnet) GetClientGroupNetworkIP(c *hivesim.Client) (string, error) {
	for k := range t.groupNetworks {
		if network, found := t.groupNetworks[k]; found {
			if ip, found := t.networks[network].ClientIP(c.Container); found {
				return ip, nil
			}
		}
	}
	return "", errors.New("Client not part of any Group Network")
}

func (t *Testnet) GenesisTime() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

// Returns false when the finality check is not successful.
// Returns true when the finality check is successful or not ready to make a decision.
func (t *Testnet) CheckSlotFinality(ctx context.Context, tim time.Time) bool {
	// start polling after first slot of genesis
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	if tim.Before(t.GenesisTime().Add(slotDuration)) {
		t.T.Logf("time till genesis: %s", t.GenesisTime().Sub(tim))
		return true
	}

	t.beaconSlots = make([]*common.Slot, 0)

	// new slot, log and check status of all beacon nodes
	var wg sync.WaitGroup
	for _, b := range t.beacons {
		wg.Add(1)
		go func(ctx context.Context, b *BeaconNode) {
			defer wg.Done()
			ctx, _ = context.WithTimeout(ctx, time.Second*5)

			var headInfo eth2api.BeaconBlockHeaderAndInfo
			if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
				t.T.Logf("[%v] failed to poll head: %v", b.Container, err)
				return
			} else if !exists {
				t.T.Fatalf("[%v] no head block", b.Container)
			}

			var out eth2api.FinalityCheckpoints
			if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &out); err != nil {
				t.T.Logf("[%v] failed to poll finality checkpoint: %v", b.Container, err)
				return
			} else if !exists {
				t.T.Fatalf("[%v] Expected state for head block", b.Container)
			}

			slot := headInfo.Header.Message.Slot
			t.T.Logf("%v: head block root %s, slot %d, justified %s, finalized %s",
				b.Container, headInfo.Root, slot, &out.CurrentJustified, &out.Finalized)
			t.beaconSlots = append(t.beaconSlots, &slot)

			if ep := t.spec.SlotToEpoch(slot); ep > out.Finalized.Epoch+2 {
				t.T.Errorf("failing to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, out.Finalized.Epoch)
			}
		}(ctx, b)
	}
	wg.Wait()
	// if the test is marked as failed, return false
	if t.T.Failed() {
		return false
	}
	return true
}

func (t *Testnet) TrackFinality(ctx context.Context) {
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	defer t.RemoveNetworks()

	for {
		select {
		case <-ctx.Done():
			// Context finalized, remove Networks and exit.
			return
		case tim := <-timer.C:
			// Check finality and update current information for verification
			// mechanisms to consume
			if !t.CheckSlotFinality(ctx, tim) {
				return
			}

			// Execute all verification mechanisms for this testnet
			for _, f := range t.slotExecutableVerifications {
				f(t)
			}

			if t.testnetMustFinishSignal {
				// Signal to finish the test has been raised
				return
			}
		}
	}

	// Finally

}

func FinalEpochVerification(endEpoch common.Epoch) func(*Testnet) {
	return func(t *Testnet) {
		if len(t.beaconSlots) == 0 {
			// We haven't reached genesis
			return
		}
		for _, s := range t.beaconSlots {
			if t.spec.SlotToEpoch(*s) >= endEpoch {
				t.T.Logf("End Epoch %v has been reached, finshing testnet", t.spec.SlotToEpoch(*s))
				t.testnetMustFinishSignal = true
				return
			}
		}
	}
}
