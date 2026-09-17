package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
	mpplib "github.com/tempoxyz/mpp-go/pkg/mpp"
	"github.com/tempoxyz/mpp-go/pkg/server"
	charge "github.com/tempoxyz/mpp-go/pkg/tempo/server"
)

const paymentScheme = "Payment "

type mppLane struct {
	payment   *server.Mpp
	amount    string
	recipient string
}

func newMPPLane(cfg config.Config) (Lane, error) {
	secret := strings.TrimSpace(cfg.MPPSecretKey)
	if secret == "" {
		return nil, fmt.Errorf("MPP_SECRET_KEY is required")
	}
	recipient := strings.TrimSpace(cfg.MPPRecipient)
	if recipient == "" {
		return nil, fmt.Errorf("MPP_RECIPIENT is required when MPP_SECRET_KEY is set")
	}
	rpcURL := strings.TrimSpace(cfg.MPPRPCURL)
	if rpcURL == "" {
		rpcURL = "https://rpc.moderato.tempo.xyz"
	}
	realm := strings.TrimSpace(cfg.MPPRealm)
	if realm == "" {
		realm = "klimatsearch"
	}
	amount := strings.TrimSpace(cfg.MPPAmount)
	if amount == "" {
		amount = "0.01"
	}
	method, err := charge.MethodFromConfig(charge.Config{
		RPCURL:    rpcURL,
		Recipient: recipient,
	})
	if err != nil {
		return nil, err
	}
	return &mppLane{
		payment:   server.New(method, realm, secret),
		amount:    amount,
		recipient: recipient,
	}, nil
}

func (m *mppLane) Present(r *http.Request) bool {
	_, ok := paymentAuthorization(r)
	return ok
}

func (m *mppLane) Check(r *http.Request) Decision {
	body := drainBody(r)
	params := server.ChargeParams{
		Authorization: r.Header.Get("Authorization"),
		Amount:        m.amount,
		Recipient:     m.recipient,
		Description:   "klimatsearch",
		ExternalID:    strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		MppxScope:     server.ScopeFromHTTPRequest(r, ""),
	}
	if len(body) > 0 {
		params.Body = body
	}
	result, err := m.payment.Charge(r.Context(), params)
	if err != nil {
		return mppErrorDecision(err)
	}
	if result == nil || result.IsChallenge() {
		if result == nil || result.Challenge == nil {
			return mppErrorDecision(mpplib.ErrPaymentRequired(m.payment.Realm(), "klimatsearch"))
		}
		return challengeDecision(m.payment, result.Challenge)
	}
	h := make(http.Header)
	h.Set("Cache-Control", "private")
	if result.Receipt != nil {
		h.Set("Payment-Receipt", result.Receipt.ToPaymentReceipt())
	}
	return Decision{OK: true, Header: h}
}

func paymentAuthorization(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < len(paymentScheme) || !strings.EqualFold(h[:len(paymentScheme)], paymentScheme) {
		return "", false
	}
	return strings.TrimSpace(h[len(paymentScheme):]), true
}

func challengeDecision(payment *server.Mpp, ch *mpplib.Challenge) Decision {
	h := make(http.Header)
	h.Set("Content-Type", "application/problem+json")
	h.Set("Cache-Control", "no-store")
	header, err := ch.ToAuthenticateStrict(payment.Realm())
	if err != nil {
		return mppErrorDecision(mpplib.ErrBadRequest(err.Error()))
	}
	h.Set("WWW-Authenticate", header)
	pe := mpplib.ErrPaymentRequired(payment.Realm(), ch.Description)
	b, _ := json.Marshal(pe.ProblemDetails(""))
	return Decision{Status: http.StatusPaymentRequired, Header: h, Body: b}
}

func mppErrorDecision(err error) Decision {
	h := make(http.Header)
	h.Set("Content-Type", "application/problem+json")
	h.Set("Cache-Control", "no-store")
	var pe *mpplib.PaymentError
	if !errors.As(err, &pe) {
		pe = mpplib.ErrVerificationFailed(err.Error())
	}
	b, _ := json.Marshal(pe.ProblemDetails(""))
	return Decision{Status: pe.Status, Header: h, Body: b}
}
