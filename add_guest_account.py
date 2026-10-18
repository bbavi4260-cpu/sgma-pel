#!/usr/bin/env python3
"""Add new guest account(s) to data/sigma.db for multiplayer testing.

    python3 add_guest_account.py                # one new guest
    python3 add_guest_account.py --count 3      # three
    python3 add_guest_account.py --name PLAYER2 # custom display name
    python3 add_guest_account.py --list         # show every account in the database

Each account gets a NEW account id, open id, guest access/refresh token and session token. A new
device's first guest register is handed the next unclaimed account automatically. The server does not
need a restart (accounts are re-read from the database on every login / matchmaking auth).
"""
from __future__ import annotations

import argparse
from pathlib import Path

import test_server

ROOT = Path(__file__).resolve().parent


def show(account: dict) -> None:
    guest = account.get("guest") or {}
    print(f"  account_id    : {account['account_id']}")
    print(f"  display_name  : {account.get('display_name')}")
    print(f"  open_id       : {account.get('open_id')}")
    print(f"  session_token : {account.get('session_token')}")
    if guest:
        print(f"  guest token   : {guest.get('access_token')}")
        print(f"  guest refresh : {guest.get('refresh_token')}")
        print(f"  claimed       : {guest.get('claimed')}")
    print()


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--count", type=int, default=1)
    ap.add_argument("--name", default=None)
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--root", default=str(ROOT))
    args = ap.parse_args()
    root = Path(args.root).resolve()
    store = test_server.SQLiteAccountStore(
        root / "data" / "sigma.db", root / "data" / "account_state.json",
        root / "resources" / "source_default_profiles.json", root / "resources" / "available_maps.json")
    if args.list:
        for account in store.all_accounts():
            show(account)
        return
    for i in range(max(1, args.count)):
        name = args.name if args.name and args.count == 1 else (f"{args.name}{i + 1}" if args.name else None)
        print("NEW GUEST ACCOUNT ADDED:")
        show(store.create_guest(name, claimed=False))


if __name__ == "__main__":
    main()
