package guard

import (
	"bytes"
	"io"
	"net/http"

	sellers "cdn.zeroclick.io/sdks/sellers-go"

	"github.com/mong-x/klimatsearch/internal/config"
)

const zcServiceSlug = "klimatsearch"

// zeroClickLane uses the ZeroClick sellers SDK Guard/Verify.
// Real path: cdn.zeroclick.io/sdks/sellers-go Client.Guard / Verify.
type zeroClickLane struct {
	client  *sellers.Client
	secrets map[string]string
}

func newZeroClickLane(cfg config.Config) (Lane, error) {
	secrets, err := sellers.ParseSigningSecrets(cfg.ZeroClickSigningSecrets)
	if err != nil {
		return nil, err
	}
	lane := &zeroClickLane{secrets: secrets}
	if cfg.ZeroClickAPIKey != "" {
		client, err := sellers.New(sellers.Config{
			APIKey:         cfg.ZeroClickAPIKey,
			ServiceSlug:    zcServiceSlug,
			SigningSecrets: secrets,
		})
		if err != nil {
			return nil, err
		}
		lane.client = client
	}
	return lane, nil
}

func (z *zeroClickLane) Present(r *http.Request) bool {
	return r.Header.Get("zc-signature") != ""
}

func (z *zeroClickLane) Check(r *http.Request) Decision {
	body := drainBody(r)
	req := sellers.FromHTTP(r, body)
	if z.client != nil {
		result, err := z.client.Guard(r.Context(), req, zcServiceSlug, []sellers.UsageItem{sellers.PerRequest("requests", 1)}, "")
		if err != nil {
			return deny(http.StatusUnauthorized, "unauthorized")
		}
		if !result.OK {
			return zcDecision(result.Response)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		return Decision{OK: true}
	}
	result, err := sellers.Verify(req, sellers.VerifyOptions{Resolve: sellers.SigningSecrets(z.secrets)})
	if err != nil {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	if !result.OK {
		return zcDecision(result.Response)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return Decision{OK: true}
}

func zcDecision(resp *sellers.Response) Decision {
	if resp == nil {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	h := make(http.Header)
	for k, v := range resp.Headers {
		h.Set(k, v)
	}
	return Decision{Status: resp.Status, Header: h, Body: resp.Body}
}
