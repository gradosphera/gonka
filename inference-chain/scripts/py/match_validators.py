#!/usr/bin/env python3
import requests
import base64
import hashlib
import sys
from dataclasses import dataclass

DEFAULT_RPC = "http://node2.gonka.ai:26657"   # default RPC endpoint
HEIGHT = 2


@dataclass
class Validator:
    pub_key: str
    voting_power: int


@dataclass
class Participant:
    address: str
    validator_key: str


def get_rpc_from_args() -> str:
    """Return RPC endpoint from CLI args or default if not provided."""
    if len(sys.argv) > 1:
        return sys.argv[1]
    return DEFAULT_RPC


def get_genesis(rpc: str) -> dict:
    res = requests.get(f"{rpc}/genesis")
    res.raise_for_status()
    return res.json()["result"]["genesis"]


def extract_validators_from_genesis(genesis) -> list[Validator]:
    """Extract validators from the genesis gen_txs section."""
    validators = []
    for tx in genesis["app_state"]["genutil"]["gen_txs"]:
        for msg in tx["body"]["messages"]:
            if msg["@type"] != "/cosmos.staking.v1beta1.MsgCreateValidator":
                continue
            v = Validator(
                pub_key=msg["pubkey"]["key"],
                voting_power=int(msg["value"]["amount"]),
            )
            validators.append(v)
    return validators


def extract_participants_from_genesis(genesis) -> list[Participant]:
    """Extract participants from the custom inference module."""
    participants = []
    inference = genesis["app_state"].get("inference", {})
    for p in inference.get("participant_list", []):
        if not p.get("validator_key"):
            continue
        participants.append(
            Participant(address=p.get("address", ""), validator_key=p["validator_key"])
        )
    return participants


def tm_val_address_from_pubkey_b64(b64key: str) -> str:
    """Compute Tendermint validator address = first 20 bytes of SHA256(pubkey)."""
    pk_bytes = base64.b64decode(b64key)
    sha = hashlib.sha256(pk_bytes).digest()
    return sha[:20].hex().upper()


def main():
    rpc = get_rpc_from_args()
    print(f"Using RPC endpoint: {rpc}")

    # Step 1: read genesis
    genesis = get_genesis(rpc)

    # Step 2: extract validators and participants
    validators = extract_validators_from_genesis(genesis)
    participants = extract_participants_from_genesis(genesis)

    if not validators:
        sys.exit("ERROR: no validators found in genesis gen_txs")
    if not participants:
        sys.exit("ERROR: no participants found in app_state.inference")

    # Step 3: build mapping pubkey -> validator address
    validator_mapping = {
        v.pub_key: tm_val_address_from_pubkey_b64(v.pub_key)
        for v in validators
    }

    # Step 4: match participants to validators
    unmatched = []
    matched = []

    for p in participants:
        if p.validator_key in validator_mapping:
            matched.append(
                (p.address, validator_mapping[p.validator_key], p.validator_key)
            )
        else:
            unmatched.append(p.address)

    # Step 5: verify and print result
    if unmatched:
        sys.exit(f"ERROR: could not match validator keys for participants: {unmatched}")

    print("All participants successfully matched:")
    for addr, val_hex, pubkey in matched:
        print(f"{addr} -> {val_hex} -> {pubkey}")


if __name__ == "__main__":
    main()
