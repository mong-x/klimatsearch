// Package compare resolves two Resource IDs into one Comparison. REST, the
// operator console, and MCP render the result; this module owns the
// load-both-and-compare envelope and its error taxonomy.
package compare

import (
	"context"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Getter loads one Resource by Resource ID or catalog:id. *store.Store
// satisfies it; tests use a fake so the taxonomy is pinnable without CGO.
type Getter interface {
	Get(ctx context.Context, id string) (*model.Resource, error)
}

// SideError marks which side of a Comparison failed to load: "a" or "b".
// The side is structured so each tier renders its own prefix (MCP's
// "id_a:") instead of inheriting a baked-in one.
type SideError struct {
	Side string
	Err  error
}

func (e *SideError) Error() string { return e.Err.Error() }
func (e *SideError) Unwrap() error { return e.Err }

// Resources loads both Resources and compares them. Load failures come
// back as a SideError wrapping the store error; everything else is
// model.Compare's own typed errors.
func Resources(ctx context.Context, g Getter, idA, idB, source, unit, impact string) (model.Comparison, error) {
	a, err := g.Get(ctx, idA)
	if err != nil {
		return model.Comparison{}, &SideError{Side: "a", Err: err}
	}
	b, err := g.Get(ctx, idB)
	if err != nil {
		return model.Comparison{}, &SideError{Side: "b", Err: err}
	}
	return model.Compare(*a, *b, source, unit, impact)
}
