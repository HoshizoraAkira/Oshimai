# Chaos fault injection

Oshimai injects faults through a single `ChaosDriver` interface (`Apply` / `Revert` / `Status`,
[`pkg/chaos/driver.go`](../pkg/chaos/driver.go)) with four interchangeable implementations. Every
driver is wrapped in `ManagedChaosDriver`, which is where the actual safety guarantee lives — see
[Dead-man switch](#dead-man-switch-safety-guarantee) below.

## Drivers

Selected with `-chaos-driver` on `oshimai-server` (or `CHAOS_DRIVER` in `.env`):

| Driver | Requirements | Notes |
|---|---|---|
| `mock` | none | Safe default — simulates faults with no side effects. Use this for CI and for testing scenarios/dashboards without touching real infrastructure. |
| `http_proxy` | none | A no-root userspace HTTP proxy your Virtual Users route traffic through. Works on any OS; supports latency/jitter/loss/corruption, DNS faults, and clock-skew simulation (see below). |
| `netem` | Linux + `NET_ADMIN` | Real kernel-level network fault injection via `tc netem` — the most realistic option, but Linux-only and needs elevated privileges. |
| `resource_stress` | none | CPU/memory/disk pressure on the host running `oshimai-server` itself, rather than the network path. |

## Fault parameters

A fault (`FaultSpec`, [`pkg/chaos/model.go`](../pkg/chaos/model.go)) targets traffic via `Filter`
(interface, IP, port, protocol, or domain list) and combines any of:

| Field | Meaning |
|---|---|
| `latency` / `jitter` / `correlation` | Injected delay, its variance, and how correlated consecutive delays are (0.0-1.0). |
| `loss_percent` | Chance a packet/request is dropped outright. |
| `corruption_percent` | Chance of payload corruption. |
| `rate_limit_kbps` | Bandwidth cap. |
| `dns_fail_percent` / `dns_delay` | DNS resolution failure chance / extra resolution delay (`http_proxy` only). |
| `clock_skew_offset` | Injects an `X-Oshimai-Simulated-Time` header with a skewed timestamp — an app-level simulation for cooperating targets, since changing the real OS clock needs root (`http_proxy` only). |
| `duration` | **Mandatory** — the fault's TTL. This is what the dead-man switch below arms itself against. |

## Dead-man switch (safety guarantee)

Every fault, regardless of driver, is auto-reverted by three independent mechanisms so a forgotten
or crashed experiment can never leave a real fault permanently applied:

1. **Watchdog timer** — arms for exactly `fault.duration` on `Apply`; fires `Revert` on its own even if nothing else does.
2. **Context cancellation** — if the run's context is cancelled (aborted, circuit-breaker trip) before the watchdog fires, `Revert` runs immediately instead of waiting out the full duration.
3. **OS signal trap** — `SIGINT`/`SIGTERM` (e.g. an operator hitting Ctrl+C, or a graceful `oshimai-server` shutdown) triggers `Revert` before the process exits.

All three converge on the same idempotent `Revert`, so it's always safe to trigger more than one.

## Kubernetes-native chaos

[`pkg/k8schaos`](../pkg/k8schaos) implements the same `ChaosDriver` interface against a real
Kubernetes cluster over plain REST+JSON (no `client-go` dependency), so it gets the identical
dead-man-switch guarantee for free when wrapped in `ManagedChaosDriver`:

- **Pod-kill** (`k8s-pod-kill`) — deletes a percentage of running pods matching a label selector. "Revert" is necessarily a no-op (you can't un-kill a pod); the point is watching the pod's controller — Deployment, StatefulSet — reschedule it.
- **HPA autoscaler watch** (`k8s-autoscaler`) — samples a HorizontalPodAutoscaler's replica count over a window while you run load against it, and verdicts whether/how fast it reacted.

```bash
oshimai k8s-pod-kill -k8s-api-server https://10.0.0.1:6443 -k8s-token $TOKEN -selector app=checkout -percent 50 -duration 30s
oshimai k8s-autoscaler -k8s-api-server https://10.0.0.1:6443 -k8s-token $TOKEN -hpa checkout-hpa -watch 60s
```

Run `oshimai help` for the full flag list, or see [docs/cli.md](cli.md) for the complete CLI
reference.
