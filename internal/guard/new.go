//go:build !hosted

package guard

import (
	"log/slog"
	"os"

	"github.com/mong-x/klimatsearch/internal/config"
)

// New is a no-op Guard unless the operator sets KLIMAT_API_KEYS, in which
// case requests must carry one of those self-managed keys. Self-hosted
// builds never link Unkey or MPP.
func New(cfg config.Config) (*Guard, error) {
	if os.Getenv("UNKEY_ROOT_KEY") != "" || os.Getenv("MPP_SECRET_KEY") != "" {
		slog.Warn("this binary was not built with -tags hosted; Unkey/MPP configuration is ignored")
	}
	if lane := newStaticLane(cfg.APIKeys); lane != nil {
		return &Guard{Static: lane}, nil
	}
	return &Guard{AllowAll: true}, nil
}
