# terraform-provider-oshimai

Manage recurring GameDay chaos/load schedules on an Oshimai server as Terraform resources, right next to the infrastructure they test.

A separate Go module from the main `github.com/oshimai/twin` codebase — the Terraform Plugin Framework brings a large dependency tree (gRPC, OpenTelemetry, `go-plugin`) that has no business bloating the server's own `go.mod`.

## Example

```hcl
terraform {
  required_providers {
    oshimai = {
      source = "oshimai/oshimai"
    }
  }
}

provider "oshimai" {
  server_url = "https://oshimai.internal.example.com"
}

resource "oshimai_gameday_schedule" "checkout_quarterly" {
  name       = "checkout-quarterly-gameday"
  weekday    = 2  # Tuesday
  hour_utc   = 9
  minute_utc = 0

  run_request_json = jsonencode({
    scenario_yaml = file("${path.module}/scenarios/checkout.yaml")
    load_config = {
      profile  = "flat_vu"
      vus      = 20
      duration = 300000000000 # 5 minutes, in nanoseconds
    }
  })
}
```

## Build

```bash
cd tools/terraform-provider-oshimai
go build -o terraform-provider-oshimai
```

To use a locally built binary without publishing to a registry, add a [`dev_overrides`](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) block to your Terraform CLI config pointing at this directory.

## Notes on the resource model

The Oshimai server has no `GET /{id}` or `PATCH` route for schedules, which shapes how `oshimai_gameday_schedule` behaves:

- **Read** lists every schedule and matches by id (fine at GameDay-schedule scale — this is not a high-cardinality resource).
- **Update** only supports flipping `enabled` in place, via the `/toggle` endpoint. Changing `name`, `weekday`, `hour_utc`, `minute_utc`, or `run_request_json` replaces the resource (delete + recreate), since the server has no in-place edit for those fields.
