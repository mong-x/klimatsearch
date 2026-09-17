//go:build hosted

package guard

import (
	"net/http"

	unkey "github.com/unkeyed/sdks/api/go/v2"
	"github.com/unkeyed/sdks/api/go/v2/models/components"
)

type unkeyLane struct {
	client *unkey.Unkey
}

func newUnkeyLane(rootKey string) Lane {
	return &unkeyLane{client: unkey.New(unkey.WithSecurity(rootKey))}
}

func (u *unkeyLane) Present(r *http.Request) bool {
	_, ok := bearerToken(r)
	return ok
}

func (u *unkeyLane) Check(r *http.Request) Decision {
	tok, ok := bearerToken(r)
	if !ok {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	res, err := u.client.Keys.VerifyKey(r.Context(), components.V2KeysVerifyKeyRequestBody{Key: tok})
	if err != nil {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	if res == nil || res.V2KeysVerifyKeyResponseBody == nil || !res.V2KeysVerifyKeyResponseBody.Data.Valid {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	return Decision{OK: true}
}
