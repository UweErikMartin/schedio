package customer

import (
	"time"

	"schedio/internal/store"
)

// parseTimezone resolves an IANA timezone name (e.g. "Europe/Berlin") to a
// *time.Location. If the name is empty or invalid, time.UTC is returned so
// that email rendering always has a defined timezone.
func parseTimezone(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// inTZ returns a copy of each Booking in bs with StartAt and EndAt converted
// to the given location. This allows Go text/template's .Format call to render
// times in the correct timezone without any template changes.
func inTZ(bs []*store.Booking, loc *time.Location) []*store.Booking {
	if loc == nil {
		loc = time.UTC
	}
	out := make([]*store.Booking, len(bs))
	for i, b := range bs {
		cp := *b
		cp.StartAt = b.StartAt.In(loc)
		cp.EndAt = b.EndAt.In(loc)
		out[i] = &cp
	}
	return out
}

// inTZSingle returns a copy of b with StartAt and EndAt converted to loc.
func inTZSingle(b *store.Booking, loc *time.Location) *store.Booking {
	if loc == nil {
		loc = time.UTC
	}
	cp := *b
	cp.StartAt = b.StartAt.In(loc)
	cp.EndAt = b.EndAt.In(loc)
	return &cp
}
