//go:build !hosted

package guard

// Guard is a pass-through in the self-hosted build.
type Guard struct {
	AllowAll bool
}
