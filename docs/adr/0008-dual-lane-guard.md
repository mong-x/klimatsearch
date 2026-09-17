# Dual-lane Guard: MPP then Unkey

Paid endpoints (`/api/search`, resource read/compare, MCP, admin ingest) go through one Guard:

1. If `Authorization: Payment …` is present, verify with official `github.com/tempoxyz/mpp-go` v0.2.0 (`pkg/tempo/server` `MethodFromConfig` + `pkg/server` `Charge`). ChargeParams uses `Authorization` (not a separate Payment-Authorization header).
2. Else if `Authorization: Bearer` is present, verify with Unkey `github.com/unkeyed/sdks/api/go/v2` `Keys.VerifyKey`.
3. Else if the MPP lane is configured, issue HTTP 402 with `WWW-Authenticate: Payment` (empty credential needs no RPC). Else 401.

Healthz stays public. When Unkey and MPP secrets are both unset, the Guard allows all traffic so `make run` and kind e2e keep working; the moment either secret is set, the Guard is fail-closed. `MPP_RECIPIENT` is required with `MPP_SECRET_KEY`. Unkey meters to Stripe; MPP settles on Tempo.
