# Fix summary — 2026-10-12

## Mode screen: “Reading game info, please wait”

The supplied client TCP schema defines matchmaking subcommand `7` as `GAMEOPENINGINFO`, with a `GameOpeningInfoReq` carrying region and language. The observed `payload_bytes=8` rejection occurred because the gateway accepted only START (`1`) and CANCEL (`2`), so the mode-selection request was discarded. The gateway now accepts cmd 7 and returns a valid `GameOpeningInfoRes` containing an explicitly present but empty opening-info list. No timed event rows or numeric map IDs are fabricated. A new TCP regression test authenticates, sends cmd 7, and verifies the response envelope and nested response bytes.

## Billboard

Billboard responses now start 30 days before server time by default, with `SIGMA_BILLBOARD_START_GRACE_SECONDS` available to adjust the clock-skew allowance. Both login and lobby-refresh routes generate a fresh response each time. The supplied `proto.proto` confirms the field numbers used by the response. This cannot force the client to request or redraw a marquee if its UI caches it.

## UDP and remaining match limitation

The UDP server recognizes protocol command `448` (`RUDP_NOPARAM_NOTIFY`) as a notification that needs no response. Raw unknown UDP payloads and failed cmd-440 token-extraction payloads are no longer written to logs, preventing session tokens from leaking into console output. A Go regression test covers cmd 448 before session setup.

The screenshot and logs show Battle Royale selected (`game_mode=41`). The included Go match loop implements Contra Squad, not a complete Battle Royale world/session. The captured UDP excerpt shows HELLO, acknowledgement, and cmd 448 with a 646-byte payload, but no cmd 439/440 JOIN packet or cmd 100/101/130 join response. This archive does not claim a complete Battle Royale gameplay fix.

## Validation

Python tests: 16 passed. Node tests: 6 passed. Node syntax check and Python syntax compilation passed. A Go regression test was added but could not be run in this Sandbox because Go is not installed; run `cd match-server && go test ./...` in Termux after deploying.
