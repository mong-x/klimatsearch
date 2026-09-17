//go:build hosted

package guard

import (
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
)

// New builds a Guard from process config.
// No Unkey and no MPP secret → AllowAll.
// Either set → fail-closed.
func New(cfg config.Config) (*Guard, error) {
	unkeyKey := strings.TrimSpace(cfg.UnkeyRootKey)
	mppSecret := strings.TrimSpace(cfg.MPPSecretKey)
	if unkeyKey == "" && mppSecret == "" {
		return &Guard{AllowAll: true}, nil
	}
	g := &Guard{}
	if unkeyKey != "" {
		g.Unkey = newUnkeyLane(unkeyKey)
	}
	if mppSecret != "" {
		lane, err := newMPPLane(cfg)
		if err != nil {
			return nil, err
		}
		g.MPP = lane
	}
	return g, nil
}
