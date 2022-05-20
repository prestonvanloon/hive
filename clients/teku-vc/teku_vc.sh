#!/bin/bash

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/vc
mkdir -p /data/validators
mkdir -p /data/secrets


for keystore_path in /hive/input/keystores/*
do
  pubkey=$(basename "$keystore_path")
  mkdir "/data/validators/$pubkey"
  cp "/hive/input/keystores/$pubkey/keystore.json" "/data/validators/$pubkey.json"
  cp "/hive/input/secrets/$pubkey" "/data/secrets/$pubkey.txt"
done

cp -r /hive/input/secrets /data/secrets

LOG=INFO
case "$HIVE_LOGLEVEL" in
    0)   LOG=FATAL ;;
    1)   LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac

echo Starting Teku Validator Client

/opt/teku/bin/teku \
    validator-client \
    --logging="$LOG" \
    --log-destination=CONSOLE \
    --data-path=/data/vc \
    --network=auto \
    --beacon-node-api-endpoint="http://$HIVE_ETH2_BN_API_IP:$HIVE_ETH2_BN_API_PORT" \
    --validator-keys=/data/validators:/data/secrets \
    --validators-proposer-default-fee-recipient="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"
    
    # --network=/hive/input/config.yaml \
