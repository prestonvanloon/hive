#!/bin/bash

# Immediately abort the script on any error encountered
set -e

if [ ! -f "/hive/input/genesis.ssz" ]; then
    if [ -z "$HIVE_ETH2_ETH1_RPC_ADDRS" ]; then
      echo "genesis.ssz file is missing, and no Eth1 RPC addr was provided for building genesis from scratch."
      # TODO: alternative to start from weak-subjectivity-state
      exit 1
    fi
fi

mkdir -p /data/beacon
# mkdir -p /data/network

LOG=INFO
case "$HIVE_LOGLEVEL" in
    0)   LOG=FATAL ;;
    1)   LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac

echo "bootnodes: ${HIVE_ETH2_BOOTNODE_ENRS}"
bootnode_option=$([[ "$HIVE_ETH2_BOOTNODE_ENRS" == "" ]] && echo "" || echo "--p2p-discovery-bootnodes=$HIVE_ETH2_BOOTNODE_ENRS")

CONTAINER_IP=`hostname -i | awk '{print $1;}'`

metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics-address=0.0.0.0 --metrics-port=$HIVE_ETH2_METRICS_PORT --metrics-allow-origin=*")

echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret

echo Starting Teku Beacon Node

/opt/teku/bin/teku \
    --logging="$LOG" \
    --log-destination=CONSOLE \
    --eth1-deposit-contract-address="${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" \
    --data-storage-mode=PRUNE \
    --p2p-enabled=true \
    --p2p-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --p2p-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --p2p-advertised-ip="${CONTAINER_IP}" \
    --rest-api-enabled=true --rest-api-interface=0.0.0.0 --rest-api-port="${HIVE_ETH2_BN_API_PORT:-4000}" --rest-api-host-allowlist="*" --rest-api-cors-origins="*" \
    $bootnode_option \
    --eth1-endpoints="$HIVE_ETH2_ETH1_RPC_ADDRS" \
    --ee-endpoint="$HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS" \
    --data-path=/data/beacon \
    --p2p-subscribe-all-subnets-enabled=true \
    --p2p-peer-lower-bound="${HIVE_ETH2_P2P_MIN_PEERS:-1}" \
    --ee-jwt-secret-file="/jwtsecret" \
    --initial-state=/hive/input/genesis.ssz \
    --network=/hive/input/config.yaml \
    --data-storage-non-canonical-blocks-enabled=true \
    --validators-proposer-default-fee-recipient="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"

    # --testnet-dir=/data/testnet_setup \
    # bn \
    # --network-dir=/data/network \
    # $metrics_option $eth1_option $merge_option \
    # --disable-enr-auto-update=true  \
    # --port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    # --discovery-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}"  \
    # --target-peers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}" \
   #  --max-skip-slots="${HIVE_ETH2_MAX_SKIP_SLOTS:-1000}" \
