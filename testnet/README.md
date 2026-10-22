# Testnet explorer — `tbtm.plx.im`

A second explorer instance for the Bitmark **testnet**, running alongside the
mainnet explorer on the same host. Nothing about the mainnet instance changes:
the two share the PostgreSQL database (different schemas), the binaries, and the
web server, but not a single port, path, or table.

```
bitmarkd -datadir=.bitmark-tn ──RPC:19266──► bitmark-indexer  ──► postgres: bitmark, schema "testnet"
                              └─ZMQ:28432──►                                       ▲
                              └─ZMQ:28432──► bitmark-hp-gen ─────────────────────────┘
                                                     │
                                          HTTP :8089 ◄── lighttpd :443 ◄── tbtm.plx.im
```

## What differs from mainnet

| | mainnet | testnet |
|---|---|---|
| Hostname | `explorer.bitmark.co`, `explorer.plx.im` | `tbtm.plx.im` |
| bitmarkd datadir | `~server/.bitmark` | `~server/.bitmark-tn` |
| RPC port | 9266 | 19266 |
| ZMQ hashblock | `tcp://127.0.0.1:28332` | `tcp://127.0.0.1:28432` |
| Generator HTTP | `127.0.0.1:8088` | `127.0.0.1:8089` |
| Database | `bitmark`, schema `public` | `bitmark`, schema `testnet` |
| Web root | `/var/www/explorer.plx.im/html` | `/var/www/tbtm.plx.im/html` |
| Templates | `/var/www/templates/bitmark-hp-gen/` | `/var/www/templates/bitmark-hp-gen-testnet/` |
| tmux sessions | `bitmark-explorer` | `bitmark-explorer-tn`, `bitmark-indexer-tn` |
| `NETWORK` | unset (= `mainnet`) | `testnet` |

### Why a schema instead of a second database

`CREATE DATABASE` requires a role with `CREATEDB` or superuser rights, and the
`bitmark` role has neither. It *does* own the `bitmark` database, so it can
create a schema inside it without any superuser involvement. The testnet tables
therefore live in `testnet.blocks`, `testnet.transactions`, and so on, and the
services select them purely through `search_path` in the connection string:

```
postgres://bitmark:PASS@127.0.0.1:5432/bitmark?sslmode=disable&search_path=testnet
```

No query in the indexer or generator hard-codes a schema, so the same binaries
serve both chains. To remove the testnet data entirely:
`DROP SCHEMA testnet CASCADE;` — mainnet is untouched.

### Why testnet gets its own copy of the templates

The templates deployed at `/var/www/templates/bitmark-hp-gen/` have drifted from
the ones in this repository (the deployed homepage has a different `<title>` and
a stray `</a>`). Copying repo templates over them would push those unrelated
edits onto the live mainnet explorer. Giving testnet its own directory keeps the
two deployments independent; reconciling the drift is a separate decision.

The `NETWORK` environment variable is what makes one template set serve both
chains: `{{ if isTestnet }}` adds the TESTNET badge and replaces the mainnet-only
plot links. Unset, it renders exactly as before.

> **Binary compatibility**: the new `bitmark-hp-gen` renders the old deployed
> templates fine (unused template functions are harmless), so installing it does
> not disturb the running mainnet instance. The reverse is *not* true — the old
> binary cannot render a template containing `isTestnet`. Never copy the new
> templates into the mainnet directory without also restarting mainnet.

---

## Install

### 1. Testnet node — ZMQ (done)

`/home/server/.bitmark-tn/bitmark.conf` publishes on 28432:

```
zmqpubhashblock=tcp://127.0.0.1:28432
zmqpubrawtx=tcp://127.0.0.1:28432
```

Verify: `bitmark-cli -datadir=/home/server/.bitmark-tn getzmqnotifications`

### 2. Database schema (done)

Created by the `bitmark` role, no superuser:

```bash
cd /home/ak/git/Bitmark-Explorer/database
NO_SUPERUSER=1 DB_SCHEMA=testnet DB_NAME=bitmark DB_USER=bitmark DB_PASS=... \
  bash verify-schema.sh
```

Safe to re-run; it only adds missing columns and indexes.

### 3. Build and install the binaries — as root

```bash
cd /home/ak/git/Bitmark-Explorer/generator && CGO_ENABLED=1 go build -o bitmark-hp-gen ./cmd/bitmark-hp-gen
cd ../indexer                              && CGO_ENABLED=1 go build -o bitmark-indexer ./cmd/bitmark-indexer

install -m 755 ../generator/bitmark-hp-gen /usr/local/bin/bitmark-hp-gen
install -m 755 bitmark-indexer             /usr/local/bin/bitmark-indexer
```

The running mainnet processes keep their in-memory copy and are unaffected until
they are next restarted.

### 4. Web root and templates — as root

```bash
install -d -o server -g lighttpd -m 755 /var/www/tbtm.plx.im/html
install -d -o server -g lighttpd -m 755 /var/www/templates/bitmark-hp-gen-testnet

cd /home/ak/git/Bitmark-Explorer/generator/templates
install -o server -g lighttpd -m 644 homepage.html blockdetail.html \
    addressdetail.html multitx.html mpow.html /var/www/templates/bitmark-hp-gen-testnet/
install -o server -g lighttpd -m 644 bitmark128.png /var/www/tbtm.plx.im/html/
```

### 5. Config and run scripts — as root, then owned by server

```bash
cd /home/ak/git/Bitmark-Explorer/testnet
install -o server -g server -m 644 settings-testnet.conf /home/server/etc-explorer/
install -o server -g server -m 755 run-bitmark-indexer-testnet.sh \
                                   run-bitmark-hp-gen-testnet.sh /home/server/scripts/
install -d -o server -g server -m 755 /home/server/logs
```

Then create the secrets file (mode 600, never committed):

```bash
install -o server -g server -m 600 /dev/null /home/server/etc-explorer/credentials-testnet.env
cat > /home/server/etc-explorer/credentials-testnet.env <<'EOF'
RPC_USER=bitmarkrpc
RPC_PASS=<testnet rpcpassword from ~server/.bitmark-tn/bitmark.conf>
RPC_PORT=19266
DB_PASS=<same DB_PASS as the mainnet credentials.env>
DB_NAME=bitmark
DB_SCHEMA=testnet
EOF
```

### 6. Start the services — as `server`

```bash
tmux new-session -d -s bitmark-indexer-tn  /home/server/scripts/run-bitmark-indexer-testnet.sh
tmux new-session -d -s bitmark-explorer-tn /home/server/scripts/run-bitmark-hp-gen-testnet.sh

curl -sI http://127.0.0.1:8089/ | head -1        # expect: HTTP/1.1 200 OK
tmux attach -t bitmark-explorer-tn               # live TUI; Ctrl-b d to detach
```

### 7. DNS, vhost, certificate — as root

Add the DNS A record `tbtm.plx.im -> 51.83.11.113`, then follow the four
bootstrap steps documented at the top of `lighttpd-tbtm.plx.im.conf`.

---

## Operating

```bash
# Testnet indexer / generator logs and TUIs
tmux attach -t bitmark-indexer-tn
tmux attach -t bitmark-explorer-tn

# How far along is the testnet index?
psql -h 127.0.0.1 -U bitmark -d bitmark \
  -c "SELECT best_height, updated_at FROM testnet.chain_state;"

# Node vs indexed height
bitmark-cli -datadir=/home/server/.bitmark-tn getblockcount
```

**A reset testnet** (a new genesis, or a chain rewound past the indexed tip)
leaves stale rows behind. Clear them and let the indexer re-sync from scratch:

```
TRUNCATE testnet.blocks, testnet.transactions, testnet.tx_inputs,
         testnet.tx_outputs, testnet.chain_state,
         testnet.algo_stats_daily, testnet.algo_stats_current;
```
