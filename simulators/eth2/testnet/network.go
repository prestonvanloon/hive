package main

import (
	"errors"
	"sort"

	"github.com/ethereum/hive/hivesim"
)

type ClientNetwork struct {
	*hivesim.T
	// Name of the Docker Network name
	networkName string

	// Map of IPs and connection status for each ContainerID connected to this network
	clientIPs       map[string]string
	clientConnected map[string]bool
}

func NewClientNetwork(t *hivesim.T, networkName string) (*ClientNetwork, error) {
	if err := t.Sim.CreateNetwork(t.SuiteID, networkName); err != nil {
		return nil, err
	}

	return &ClientNetwork{
		T:               t,
		networkName:     networkName,
		clientIPs:       make(map[string]string),
		clientConnected: make(map[string]bool),
	}, nil
}

func (cn *ClientNetwork) RemoveNetwork() error {
	// Remove network from simulation
	return cn.Sim.RemoveNetwork(cn.SuiteID, cn.networkName)
}

func (cn *ClientNetwork) ConnectClient(containerID string) error {
	originalIp, originalIpFound := cn.clientIPs[containerID]

	// TODO: Need to somehow preserve the original IP when reconnecting to the network, API change required.
	if err := cn.Sim.ConnectContainer(cn.SuiteID, cn.networkName, containerID); err != nil {
		return err
	}
	ip, err := cn.Sim.ContainerNetworkIP(cn.SuiteID, cn.networkName, containerID)
	if err != nil {
		return err
	}

	if originalIpFound {
		if originalIp != ip {
			cn.Logf("WARN: Reconnecting client [%v] to %v changed IP %v->%v", containerID, cn.networkName, originalIp, ip)
		} else {
			cn.Logf("SUCC: Reconnecting client [%v] to %v retained IP %v==%v", containerID, cn.networkName, originalIp, ip)
		}
	}
	cn.clientIPs[containerID] = ip
	cn.clientConnected[containerID] = true

	cn.Logf("Connected [%v] to %v with IP %v", containerID, cn.networkName, ip)
	return nil
}

func (cn *ClientNetwork) DisconnectClient(containerID string) error {
	if _, clientIsMember := cn.clientIPs[containerID]; !clientIsMember {
		return errors.New("Client is not part of the network")
	}
	if err := cn.Sim.DisconnectContainer(cn.SuiteID, cn.networkName, containerID); err != nil {
		return err
	}
	cn.clientConnected[containerID] = false
	return nil
}

func (cn *ClientNetwork) ClientIP(containerID string) (string, bool) {
	ip, found := cn.clientIPs[containerID]
	return ip, found
}

func (cn *ClientNetwork) ReconnectAllClients() error {
	// Try to reconnect all clients in the same IP order:
	// it won't matter later when we have the API request with a specific IP,
	// but until then, this is the most we can do to try retain IP assignment
	type keyval struct {
		key string
		val string
	}
	var s []keyval
	for k, v := range cn.clientIPs {
		s = append(s, keyval{k, v})
	}
	sort.Slice(s, func(i int, j int) bool {
		return s[i].val < s[j].val
	})

	for _, kval := range s {
		if !cn.clientConnected[kval.key] {
			if err := cn.ConnectClient(kval.key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cn *ClientNetwork) DisconnectAllClients() error {
	for k := range cn.clientIPs {
		if cn.clientConnected[k] {
			if err := cn.DisconnectClient(k); err != nil {
				return err
			}
		}
	}
	return nil
}
