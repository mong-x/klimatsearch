//go:build !hosted

package guard

import (
	"log/slog"
	"os"

	"github.com/mong-x/klimatsearch/internal/config"
)

// New is a no-op Guard. Self-hosted builds never link Unkey or MPP.
func New(cfg config.Config) (*Guard, error) {
	_ = cfg
	if os.Getenv("UNKEY_ROOT_KEY") != "" || os.Getenv("MPP_SECRET_KEY") != "" {
		slog.Warn("this binary was not built with -tags hosted; Unkey/MPP configuration is ignored")
	}
	return &Guard{AllowAll: true}, nil
}
