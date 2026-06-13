# Lichess Bot

Noema64 does not call the Lichess API directly. Run it through a UCI-compatible bridge such as `lichess-bot`.

Build or run the UCI binary:

```sh
go build -o noema64-uci ./cmd/noema64-uci
```

Use that executable as the engine command in the bot bridge configuration. Keep verifier and provider settings in the local Noema64 config, or pass UCI options from the bridge when supported:

```text
setoption name Mode value blunderguard
setoption name LLMProvider value mock
setoption name TraceEnabled value true
```

Recommended first smoke test before connecting a bot account:

```sh
printf 'uci\nisready\nucinewgame\nposition startpos\ngo movetime 100\nquit\n' | ./noema64-uci
```

Expected behavior:

- `stdout` contains only valid UCI protocol lines.
- `bestmove` is legal or `0000` only when no legal move exists.
- JSONL traces are written locally when tracing is enabled.
- External verifier binaries are user-supplied and are not bundled with Noema64.

Pure, blunderguard, and hybrid modes should be disclosed accurately in any public bot profile. Do not describe hybrid or verifier-assisted games as pure LLM play.
