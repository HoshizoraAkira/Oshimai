# Target ownership & guarded production

Oshimai injects real load and real faults against real services — that's dangerous by design if
misused. Two independent, on-by-default gates stand between a `POST /api/v1/runs` request and
actual traffic hitting a target: **target-ownership verification** and **guarded-production
approval**. Both are enforced in [`TestCoordinator.StartRun`](../pkg/server/coordinator.go), and
both can be turned off (`-enforce-target-verification=false`, `-require-production-approval=false`)
— but deliberately only via command-line flag, never an environment variable, so a shared `.env`
file can't silently disable a safety control (see the README's Configuration table).

## 1. Target-ownership verification

The problem: a load/chaos testing tool that will happily hammer *any* URL you give it is also a
"booter-as-a-service" — the same tool used to legitimately test your own staging environment can
be pointed at someone else's production API. Oshimai's answer, modeled on how a CA validates
domain ownership before issuing a TLS certificate: **a public target must prove it's yours before
it can be tested.**

`checkTargetAllowed` ([`coordinator.go`](../pkg/server/coordinator.go)) runs this three-way check
on every run request's resolved target host:

1. **Blocklist** ([`pkg/verify/ownership.go`](../pkg/verify/ownership.go)) — a hardcoded list of
   high-traffic public domains (`google.com`, `amazonaws.com`, major Indonesian banks/e-commerce,
   etc.) that are rejected outright, regardless of any proof — nobody can legitimately "prove"
   ownership of `google.com` from inside Oshimai, so these skip the challenge flow entirely.
2. **Private/local exemption** — `localhost`, `.internal`, `.local`, `.test`, RFC1918/loopback/
   link-local addresses are always exempt. They aren't reachable from the public internet, so
   there's no third party to protect.
3. **DNS TXT / well-known challenge** — everything else must complete a challenge:

   ```bash
   oshimai verify challenge -target https://my-shop.co.id
   # Publish EITHER of:
   #   DNS TXT record: _oshimai-verify.my-shop.co.id = oshimai-verify=<token>
   #   or a file at:    my-shop.co.id/.well-known/oshimai-verify.txt containing exactly <token>
   oshimai verify confirm -target https://my-shop.co.id
   ```

   `Status()` ([`pkg/server/verification.go`](../pkg/server/verification.go)) also opportunistically
   re-checks DNS on every status poll even without an explicit `confirm` call, so a record that was
   already published (e.g. from a previous verification attempt) can self-heal a verification that
   looks unconfirmed after a server restart.

**Alternative: cloud IP ownership.** If DNS isn't practical (e.g. the target is an internal AWS
Elastic IP with no public DNS at all), `verify-cloud` proves ownership by having the *operator's
own AWS credentials* confirm the IP is allocated to their account
([`pkg/cloudverify`](../pkg/cloudverify)) — a fundamentally different proof (account-level access)
than the DNS/well-known method (control over the domain's records/webserver):

```bash
oshimai verify-cloud -target https://internal-api.example.com -aws-access-key ... -aws-secret-key ...
```

**A note on DNS propagation:** if verification keeps failing right after publishing a correct TXT
record, it's very likely DNS negative-caching upstream of wherever `oshimai-server` runs (a router
or ISP resolver that cached "this record doesn't exist" before you created it) — not a bug. Query
a public resolver directly (`dig @1.1.1.1 TXT _oshimai-verify.yourdomain.com`) to confirm the
record is actually live, then either wait out the cache or restart the resolver in the path. The
well-known-file method sidesteps DNS caching entirely if you need an immediate alternative.

## 2. Guarded production approval

Separately from *who owns* the target, a run labeled `environment: production` is held in
`awaiting_approval` status until a **second** operator signs off — the same run creator can't
approve their own production run, since `ApproveRun` just checks a non-empty approver name, not
that it differs from the creator (a lightweight four-eyes principle, not a strict one):

```bash
oshimai run -scenario ./prod-check.yaml -base-url https://api.example.com -environment production
# → "This run targets production and is held for approval. Run:"
oshimai approve -id run-... -approver alice
```

## Where this is enforced in the UI

The dashboard's Launcher wizard mirrors both gates client-side (disabling "Next"/"Start Test Run"
until a public target is verified) so an operator finds out *before* filling out a 5-step form
instead of after — but the server-side checks above are the actual authority; the UI gates are
purely a better user experience on top of them, not a replacement.
