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

	// Name of Docker Network names for Execution and Beacon Networks
	elNetworkName string
	bcNetworkName string

	// Map of IPs for the connections of the containers to the Execution and Beacon Networks
	clientElNetworkIP map[string]string
	clientBcNetworkIP map[string]string

	groupNetworks         map[int]string
	containerGroupNetwork map[string]int
}

func (t *Testnet) CreateNetworks() error {

	// Create Eth1 network
	if err := t.T.Sim.CreateNetwork(t.T.SuiteID, t.elNetworkName); err != nil {
		return err
	}

	// Create Beacon Chain network
	if err := t.T.Sim.CreateNetwork(t.T.SuiteID, t.bcNetworkName); err != nil {
		return err
	}

	return nil
}

func (t *Testnet) RemoveNetworks() error {
	// Remove Eth1 network
	if err := t.T.Sim.RemoveNetwork(t.T.SuiteID, t.elNetworkName); err != nil {
		return err
	}

	// Remove Beacon Chain network
	if err := t.T.Sim.RemoveNetwork(t.T.SuiteID, t.bcNetworkName); err != nil {
		return err
	}

	// TODO: Remove all other created networks

	return nil
}

func (t *Testnet) ConnectClientToExecutionNetwork(c *hivesim.Client) error {
	originalIp, originalIpFound := t.clientElNetworkIP[c.Container]

	// TODO: Need to somehow preserve the original IP when reconnecting to the network, API change required.
	if err := t.T.Sim.ConnectContainer(t.T.SuiteID, t.elNetworkName, c.Container); err != nil {
		return err
	}
	ip, err := t.T.Sim.ContainerNetworkIP(t.T.SuiteID, t.elNetworkName, c.Container)
	if err != nil {
		return err
	}

	if originalIpFound && originalIp != ip {
		t.T.Logf("WARN: Reconnecting client [%v] to Execution Network changed IP %v->%v", c.Container, originalIpFound, ip)

	}
	t.clientElNetworkIP[c.Container] = ip

	t.T.Logf("Connected [%v] to Execution Network with IP %v", c.Container, ip)
	return nil
}

func (t *Testnet) ConnectClientToBeaconNetwork(c *hivesim.Client) error {
	originalIp, originalIpFound := t.clientBcNetworkIP[c.Container]

	// TODO: Need to somehow preserve the original IP when reconnecting to the network, API change required.
	if err := t.T.Sim.ConnectContainer(t.T.SuiteID, t.bcNetworkName, c.Container); err != nil {
		return err
	}
	ip, err := t.T.Sim.ContainerNetworkIP(t.T.SuiteID, t.bcNetworkName, c.Container)
	if err != nil {
		return err
	}

	if originalIpFound && originalIp != ip {
		t.T.Logf("WARN: Reconnecting client [%v] to Beacon Network changed IP %v->%v", c.Container, originalIpFound, ip)

	}
	t.clientBcNetworkIP[c.Container] = ip

	t.T.Logf("Connected [%v] to Beacon Network with IP %v", c.Container, ip)
	return nil
}

func (t *Testnet) AddClientToGroupNetwork(c *hivesim.Client, groupNetworkIndex int) error {
	networkName, found := t.groupNetworks[groupNetworkIndex]
	if !found {
		// This is the first client added to the group, we need to create the network
		networkName = fmt.Sprintf("GroupNetwork%03d", groupNetworkIndex)
		if err := t.T.Sim.CreateNetwork(t.T.SuiteID, networkName); err != nil {
			return err
		}
		t.groupNetworks[groupNetworkIndex] = networkName
	}
	if err := t.T.Sim.ConnectContainer(t.T.SuiteID, networkName, c.Container); err != nil {
		return err
	}
	t.containerGroupNetwork[c.Container] = groupNetworkIndex
	ip, err := t.T.Sim.ContainerNetworkIP(t.T.SuiteID, networkName, c.Container)
	if err != nil {
		return err
	}
	t.T.Logf("Connected [%v] to network %v with IP %v", c.Container, networkName, ip)
	return nil
}

func (t *Testnet) GetClientGroupNetworkIP(c *hivesim.Client) (string, error) {
	groupNetworkId, ok := t.containerGroupNetwork[c.Container]
	if !ok {
		return "", errors.New("Client not part of any Group Network")
	}
	groupNetworkName := t.groupNetworks[groupNetworkId]
	ip, err := t.T.Sim.ContainerNetworkIP(t.T.SuiteID, groupNetworkName, c.Container)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func (t *Testnet) GenesisTime() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

func (t *Testnet) TrackFinality(ctx context.Context) {

	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)

	for {
		select {
		case <-ctx.Done():
			t.RemoveNetworks()
			return
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.T.Logf("time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes

			var wg sync.WaitGroup
			for i, b := range t.beacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconNode) {
					defer wg.Done()
					ctx, _ = context.WithTimeout(ctx, time.Second*5)

					var headInfo eth2api.BeaconBlockHeaderAndInfo
					if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
						t.T.Errorf("[beacon %d] failed to poll head: %v", i, err)
						return
					} else if !exists {
						t.T.Fatalf("[beacon %d] no head block", i)
					}

					var out eth2api.FinalityCheckpoints
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &out); err != nil {
						t.T.Errorf("[beacon %d] failed to poll finality checkpoint: %v", i, err)
						return
					} else if !exists {
						t.T.Fatalf("[beacon %d] Expected state for head block", i)
					}

					slot := headInfo.Header.Message.Slot
					t.T.Logf("beacon %d: head block root %s, slot %d, justified %s, finalized %s",
						i, headInfo.Root, slot, &out.CurrentJustified, &out.Finalized)

					if ep := t.spec.SlotToEpoch(slot); ep > out.Finalized.Epoch+2 {
						t.T.Errorf("failing to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, out.Finalized.Epoch)
					}
				}(ctx, i, b)
			}
			wg.Wait()
		}
	}
}
