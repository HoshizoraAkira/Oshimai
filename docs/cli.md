# CLI reference

`oshimai` is a thin HTTP client over the same REST/SSE API the web dashboard talks to (`pkg/server`) — every scriptable action stays in lockstep with what the UI can do. It's a separate binary from `oshimai-server`; it doesn't need to run on the same machine, it just needs network access to wherever the server is listening.

Build it with `go build -o bin/oshimai ./cmd/cli` (or `make build-go`), then point it at a running server:

```bash
./bin/oshimai run -scenario ./examples/scenario_ecommerce.yaml -vus 20 -duration 15s
```

Every command accepts a global `-server` flag (default `http://localhost:8080`). Run `oshimai help` for the full flag list per command.

## Commands

| Command | Purpose |
|---|---|
| `run` | Launch a test run from a scenario file or an OpenAPI spec, wait for completion |
| `list` | List recent runs |
| `status` | Show a run's status and diagnostics |
| `narrate` | AI-narrate a run's diagnostics, optionally cross-referenced with a repo scan and/or dependency graph |
| `abort` | Abort a running test |
| `approve` | Approve a run held for guarded-production approval |
| `generate` | Synthesize a scenario from an OpenAPI spec file |
| `presets` | Manage cultural load presets: `list \| create \| update \| delete` |
| `verify` | Target-ownership verification: `challenge \| confirm \| status` |
| `verify-cloud` | Verify target ownership via AWS IP allocation instead of DNS |
| `autofuzz` | Bisect the minimal chaos severity that breaks a scenario |
| `autopilot` | Bisect the exact safe concurrency ceiling for a scenario |
| `graph` | Mine an OTel trace file into a ranked endpoint dependency graph |
| `scan` | Scan a Go repo for resiliency anti-patterns — no server required |
| `import` | Convert a HAR/Postman/Insomnia/JMeter/k6 export into an Oshimai scenario |
| `agents` | List connected multi-region load-generation agents |
| `run-multiregion` | Fan a run out across connected agents and merge the results |
| `k8s-pod-kill` | Delete a percentage of pods matching a label selector |
| `k8s-autoscaler` | Watch an HPA's replica count during a load test and verdict its reaction time |
| `gameday` | Manage recurring GameDay schedules: `list \| create \| delete` |

## Examples

```bash
oshimai run -scenario ./examples/scenario_ecommerce.yaml -vus 20 -duration 15s
oshimai run -openapi ./api.json -base-url https://staging.example.com -target-rps 50 -duration 30s
oshimai list -limit 10
oshimai status -id run-1699999999-1
oshimai narrate -id run-1699999999-1 -scan-path . -graph-traces ./traces.json
oshimai presets create -name "Black Friday" -peak-multiplier 25 -duration-sec 240
oshimai verify challenge -target https://my-shop.co.id
oshimai verify confirm -target https://my-shop.co.id
oshimai autofuzz -scenario ./examples/scenario_ecommerce.yaml -vus 20
oshimai autopilot -scenario ./examples/scenario_ecommerce.yaml -min-vus 1 -max-vus 300
oshimai scan -path .
oshimai import -format har -file ./recording.har -out ./scenario.yaml
oshimai graph -traces ./traces.json
```

## Target-ownership verification workflow

A public target must pass ownership verification before it can be load- or chaos-tested (see [`pkg/verify`](../pkg/verify)). The CLI mirrors the same challenge/confirm flow the web dashboard's Target step uses:

```bash
oshimai verify status -target https://my-shop.co.id       # check current status first
oshimai verify challenge -target https://my-shop.co.id    # get a DNS TXT / well-known token to publish
# publish EITHER the DNS TXT record or the well-known file, then:
oshimai verify confirm -target https://my-shop.co.id
```

Private/internal targets (`localhost`, `.internal`, RFC1918 addresses, etc.) are exempt and never need this. Alternatively, `oshimai verify-cloud` proves ownership via AWS Elastic IP allocation instead of DNS.

If `oshimai run` is pointed at a public, unverified target, the server rejects it with a `400` explaining exactly which of the two steps above to run next — safe to rely on in a script since the error text is stable.

## Multi-region runs

`oshimai run-multiregion` doesn't talk to `oshimai-agent` workers directly — it calls `POST /api/v1/runs/multiregion` on the control plane, which fans the work out to whichever agents have already registered and are polling for it, then merges the results. See [`cmd/agent`](../cmd/agent) for how an agent registers/polls/reports.

```bash
oshimai agents                                              # see who's connected
oshimai run-multiregion -scenario ./scenario.yaml -vus 60 -duration 30s -regions jakarta,singapore
```
