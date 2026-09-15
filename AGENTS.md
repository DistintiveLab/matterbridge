# AGENTS.md

Matterbridge is a multi-protocol chat bridge (IRC, Slack, Discord, Mattermost, Matrix, Telegram, WhatsApp, XMPP, Zulip, and more) written in Go. It relays messages between chat networks via a common internal message format.

## Commands

Dependencies are vendored (`vendor/` is committed). Always pass `-mod=vendor` (or rely on Go's automatic vendor detection since Go 1.14+).

```bash
go build -mod=vendor .                # build binary into ./matterbridge
go test -mod=vendor ./...             # run all tests (what CI runs)
go vet -mod=vendor ./...
go run -mod=vendor . -conf matterbridge.toml   # run with a config
```

Useful run flags (see matterbridge.go): `-conf <file>` (default `matterbridge.toml`), `-debug` (or env `DEBUG=1`), `-version`, `-gops` (starts gops agent for runtime inspection).

Version metadata is injected at build time:
```bash
go build -mod=vendor -ldflags "-X github.com/42wim/matterbridge/version.GitHash=$(git log --pretty=format:'%h' -n 1)" .
```

### Lint

CI runs golangci-lint (pinned v2.13.2, config format v2) with `--new-from-rev HEAD~5 --timeout=5m` (only issues introduced in the last 5 commits fail). Config is `.golangci.yaml`; `gateway/bridgemap` is excluded from linting. The config uses `default: all` with a long disable list.

### Build tags (important)

Each bridge can be compiled out or swapped via tags in `gateway/bridgemap/b*.go`:

```bash
go build -tags whatsappmulti ...          # use whatsmeow-based WhatsApp multidevice bridge (bridge/whatsappmulti) instead of default (bridge/whatsapp)
go build -tags nomsteams,whatsappmulti ...  # exclude msteams (heavy deps), useful for low-memory builds
```

- `no<protocol>` tags (e.g. `nodiscord`, `nomsteams`, `noslack`) exclude a protocol from the binary.
- `whatsappmulti` selects the multidevice WhatsApp implementation; default build uses the legacy one. The two are mutually exclusive (`bwhatsapp.go` has `!nowhatsapp !whatsappmulti` tags).
- `cgolottie` switches Telegram `.tgs` sticker conversion from shelling out to external tools to a C library (see `bridge/helper/lottie_convert.go` vs `libtgsconverter.go`).
- Standard builds are `CGO_ENABLED=0`; Docker (alpine) and goreleaser builds confirm this.

### Docker

- `Dockerfile` - standard build
- `Dockerfile_whatsappmulti` - adds `-tags whatsappmulti`
- `tgs.Dockerfile` - includes python/lottie tooling for Telegram sticker conversion

## Architecture

### Message flow

```
protocol event → bridge/<proto> handler goroutine
    → b.Remote <- config.Message        (channel from gateway)
    → Router.handleReceive() (gateway/router.go)
        → per Gateway: ignoreMessage / modifyMessage (tengo, ReplaceMessages/ReplaceNicks regexes, emoji)
        → handleFiles (MediaServer upload/download path rewriting)
        → Gateway.SendMessage() to every other bridge in the gateway
            → Bridger.Send() on the destination bridge
```

The Router also handles control events routed as messages: `EventFailure` (triggers bridge reconnect), `EventRejoinChannels`, `EventGetChannelMembers`.

### Core types

- `bridge/bridge.go` - `Bridger` interface (`Send`, `Connect`, `JoinChannel`, `Disconnect`) and `Factory func(*Config) Bridger`. `Bridge` embeds `sync.RWMutex` and provides config getters.
- `bridge/config/config.go` - `config.Message` is the universal message struct all bridges translate to/from. Events are string constants (`EventJoinLeave`, `EventMsgDelete`, `EventUserTyping`, ...). Extra payloads (files, channel members) travel in `msg.Extra map[string][]interface{}` (e.g. `msg.Extra["file"]` → `config.FileInfo`).
- `gateway/router.go` - `Router` owns all `Gateway`s, deduplicates bridge instances across gateways, starts them, and runs the central receive loop.
- `gateway/gateway.go` - `Gateway` maps channels across bridges, keeps an LRU cache (`gw.Messages`) of `"protocol msgID"` → downstream message IDs, used to propagate edits/deletes as replies (see `FindCanonicalMsgID`, `BrMsgID`).

### Key directories

- `bridge/<protocol>/` - one package per protocol. Typical files: `<proto>.go` (factory `New`, `Connect`, `Send`), `handlers.go` (incoming event handlers that push into `b.Remote`), `helpers.go` (misc utilities). Sub-packages allowed (e.g. `bridge/discord/transmitter`).
- `gateway/bridgemap/` - registry: one small file per protocol registering `FullMap["<protocol>"] = b<proto>.New` in `init()`, guarded by build tags. `gateway/bridgemap/bridgemap.go` defines `FullMap` and `UserTypingSupport`.
- `gateway/samechannel/` - config-less gateway that bridges same-named channels across all configured accounts.
- `bridge/helper/` - shared helpers for avatar/file download, image conversion, etc. Used by many bridges.
- `matterhook/` - standalone Slack/Mattermost incoming/outgoing webhook client.
- `internal/` - generated bindata (tengo script); `internal/tengo/` - tengo scripting integration.
- `contrib/` - example tengo scripts, systemd/openrc service files.
- `matterclient/` - only a README; the Mattermost client lives in the separate `matterbridge/matterclient` repo (vendored as a module).
- `hook/rockethook` - Rocket.Chat outgoing-webhook client used by the rocketchat bridge.

### Config system

- TOML (viper-based, keys case-insensitive). Samples: `matterbridge.toml.sample` (full), `.simple`, `.multi`.
- Account names are `protocol.name` (e.g. `irc.libera`); `bridge.New` splits on `.` and `log.Fatalf`s if the format is wrong. The account prefix doubles as the config section key.
- Bridges read settings via `b.GetString("Token")` etc. from `*bridge.Config`; these look up `<account>.<key>` first, then fall back to `general.<key>`. Per-protocol options are documented as comments on the `config.Protocol` struct fields.
- `RemoteNickFormat` supports placeholders: `{NICK}`, `{BRIDGE}`, `{PROTOCOL}`, `{GATEWAY}`, `{LABEL}`, `{USERID}`, `{CHANNEL}`, `{NOPINGNICK}` (inserts zero-width char to break pings), `{TENGO}`.
- Tengo scripts can hook message flow (`Tengo.InMessage`, `RemoteNickFormat` with `{TENGO}`); examples in `contrib/*.tengo`.

## Adding a new bridge

1. Create `bridge/<proto>/` package implementing `bridge.Bridger` (look at a small bridge like `zulip` or `sshchat` for the minimal pattern; the package is conventionally named `b<proto>`, e.g. `package bslack`).
2. Set `Remote: cfg.Remote` usage: incoming messages must be pushed onto `b.Remote` (a `chan config.Message`).
3. Add `gateway/bridgemap/b<proto>.go` with `// +build !no<proto>` and an `init()` registering the factory in `FullMap`.
4. Document new config options as fields+comments in `config.Protocol` (bridge/config/config.go).
5. Note: `gateway/bridgemap` files still use old-style `// +build` lines only; keep the existing style there.

## Testing

Only a handful of packages have tests: `gateway`, `gateway/samechannel`, `bridge/helper`, `bridge/discord`, `bridge/slack`, `bridge/matrix`. There is no mocking framework for protocol APIs; bridge tests cover pure helpers. Gateway tests (`gateway/gateway_test.go`) exercise routing logic with fake bridges.

## Gotchas

- **Vendor dir is committed.** After changing `go.mod`, run `go mod vendor` and commit the vendor diff, otherwise `-mod=vendor` builds fail.
- **Don't send to the origin channel**: `Gateway.SendMessage` skips the channel the message came from; avatar-download events are inverted (only sent to origin). Preserve this when touching routing.
- **Message ID caching is edit/delete machinery**: bridges that support edits return the new message ID from `Send()`; the gateway stores mappings so later edit/delete events can fan out. Changes here affect `EventMsgDelete`/edit propagation.
- **Debug logging**: `DEBUG=1` env var is equivalent to `-debug` and enables caller info in logs.
- **CI Go version is 1.26.x** (`.github/workflows/development.yml`), matching the `go 1.26.0` directive in go.mod (required by whatsmeow). Avoid stdlib/features newer than that.
- **golangci-lint**: config is v2 format; the pinned CI version must be built with a Go >= the module's `go` directive or linting fails hard.
- Leftover runtime artifacts may appear in the repo root (e.g. `session-*.gob.db`, `codigoqr.txt`) - these are WhatsApp session files created by running the binary locally, not source.
- The `api` "protocol" (`bridge/api`) exposes a REST/WebSocket interface to inject/extract messages; `apiProtocol = "api"` gets special-casing in gateway message modification.
- `MattermostPlugin` channel on the Router is for running matterbridge as a Mattermost plugin (messages routed outside the normal bridge flow).
