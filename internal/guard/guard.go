//go:build !hosted

package guard

// Guard gates the self-hosted deployment. With no KLIMAT_API_KEYS it is a
// pass-through; with keys set every non-public path requires one of them.
type Guard struct {
	AllowAll bool
	Static   Lane
}
