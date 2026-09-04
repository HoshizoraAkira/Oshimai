# Writing scenarios

A scenario is a YAML (or JSON) finite state machine of HTTP steps, connected by probabilistic
("Markov-style") transitions — not a flat list of requests. Each Virtual User walks this graph
independently, so a single scenario can produce different, realistic per-user journeys (browse →
checkout vs. browse → abandon cart) across a load test's population, instead of every VU replaying
an identical script in lockstep. The data model lives in [`pkg/vusession/model.go`](../pkg/vusession/model.go); execution in [`pkg/vusession/engine.go`](../pkg/vusession/engine.go).

## Minimal shape

```yaml
id: my_scenario_v1
name: My Scenario
base_url: https://api.example.com   # every step's path is relative to this
initial_step_id: step_one           # required — the VU's entry point
timeout: 15s                        # default per-request timeout, overridable per step

steps:
  step_one:
    id: step_one
    name: First Step
    request:
      method: GET
      path: /ping
    transitions:
      - target_step_id: END         # "END" (or an empty target) terminates the session
        probability: 1.0
```

## Steps

| Field | Purpose |
|---|---|
| `request` | `method`, `path`, `headers`, `body`, `timeout` — the actual HTTP call. |
| `extractors` | Pull a value out of the response into session state (see below). |
| `assertions` | Fail the step if the response doesn't match (see below). |
| `transitions` | Where to go next, and with what probability/weight/condition. |
| `on_failure` | What to do if the step itself fails (see below). |
| `think_time` | Pause before executing — simulates a real user reading/deciding. A step with `think_time` and no `request.path` is a pure delay node with no HTTP call at all. |

## Variable extraction and interpolation

An `extractor` pulls a value out of a step's response into named session state, which later steps
reference with `${var_name}` in their `path`, `body`, or `headers`:

```yaml
extractors:
  - source: body_json      # body_json | header | status_code | regex
    path: json.username    # dot-path into the JSON body (body_json), header name (header), or regex pattern (regex)
    target_var: current_user
    default: guest          # used if extraction yields nothing
```

```yaml
request:
  path: /get?user=${current_user}&category=electronics
```

## Transitions (Markov branching)

Each step's `transitions` list is where the state machine's branching lives — this is what makes a
scenario a *population* of different journeys rather than one fixed script:

```yaml
transitions:
  - target_step_id: step_checkout
    probability: 0.80   # 80% of VUs reaching this step proceed to checkout
  - target_step_id: END
    probability: 0.20   # 20% abandon here
```

`weight` (an integer) is an alternative to `probability` for relative weighting, and `condition`
(e.g. `"status == 200"`) can route based on the step's own result instead of chance alone.

## Assertions

A step's `assertions` decide pass/fail independently of HTTP status — a 200 with the wrong body is
still a failure:

```yaml
assertions:
  - type: status_in_range   # status_in_range | body_contains | header_equals
    min_code: 200
    max_code: 299
```

## Failure handling

```yaml
on_failure:
  action: abort            # abort | retry | transition
  max_retries: 3           # only used with action: retry
  fallback_step_id: step_x # only used with action: transition
```

## Full example

See [`examples/scenario_ecommerce.yaml`](../examples/scenario_ecommerce.yaml) for a complete
4-step login → browse → cart → checkout journey with Markov branching, extractors, and an
assertion, run against `httpbin.org`. Try it directly:

```bash
oshimai run -scenario ./examples/scenario_ecommerce.yaml -vus 20 -duration 15s
```

## Generating a scenario instead of hand-writing it

You rarely need to write this YAML by hand — Oshimai can synthesize it for you (see
[`pkg/generator`](../pkg/generator)):

```bash
oshimai generate -openapi ./api.json -base-url https://staging.example.com   # from an OpenAPI spec
oshimai import -format har -file ./recording.har -out ./scenario.yaml       # from a browser recording (HAR/Postman/Insomnia/JMeter/k6)
```

The [browser extension](../tools/browser-extension) records a click-through session and exports it
as a scenario directly. The Launcher's "1-Click Auto-Discover" and "Generate from Description"
(natural language, requires `BYNARA_API_KEY`) do the same from the dashboard.
