package ingest

import "fmt"

const (
	EventCatalogUnreachable = "catalog.unreachable"
	EventIngestFailed       = "ingest.failed"
	ReasonUnreachable       = "unreachable"
	ReasonApply             = "apply"
	ReasonFetch             = "fetch"
)

// UnreachableError is a Catalog whose HTTP Ingester could not be reached
// (Boverket JSON and Excel both failed).
type UnreachableError struct {
	Catalog string
	Err     error
}

func (e *UnreachableError) Error() string {
	if e == nil {
		return "catalog unreachable"
	}
	msg := "unreachable"
	if e.Err != nil {
		msg = e.Err.Error()
	}
	if e.Catalog == "" {
		return msg
	}
	return fmt.Sprintf("%s: %s", e.Catalog, msg)
}

func (e *UnreachableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
