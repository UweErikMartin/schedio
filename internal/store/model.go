// Package store defines the data model and store interfaces for the schedio
// application. CalDAV-level types live in this file; domain-level types
// (User, Service, Timeslot, Contact, BookingSession, Booking, Settings) live
// in model_domain.go.
package store

import "time"

// Calendar represents a CalDAV calendar collection.
type Calendar struct {
	ID          string
	Name        string
	Description string
	Color       string // CSS-style color hint, e.g. "#1bafd6"
	Timezone    string // IANA timezone name, e.g. "Europe/Berlin"
}

// Event is the internal representation of a calendar event (VEVENT).
type Event struct {
	ID          string    // Globally unique identifier – mapped to iCal UID
	CalendarID  string    // ID of the owning Calendar
	Summary     string    // iCal SUMMARY
	Description string    // iCal DESCRIPTION
	Location    string    // iCal LOCATION
	Start       time.Time // iCal DTSTART (always stored in UTC)
	End         time.Time // iCal DTEND   (always stored in UTC)
	AllDay      bool      // when true DTSTART/DTEND use DATE not DATE-TIME
	Opacity     EventOpacity
	Status      EventStatus
	Organizer   Attendee
	Attendees   []Attendee
	Created     time.Time // iCal CREATED
	Modified    time.Time // iCal LAST-MODIFIED
	Sequence    int       // iCal SEQUENCE – increment on every update
	ETag        string    // opaque version token for conditional PUT / DELETE
}

// EventStatus mirrors the iCal STATUS property values for VEVENT.
type EventStatus string

const (
	StatusTentative EventStatus = "TENTATIVE"
	StatusConfirmed EventStatus = "CONFIRMED"
	StatusCancelled EventStatus = "CANCELLED"
)

// EventOpacity mirrors free/busy transparency for VEVENT.
// It is mapped to iCal TRANSP where OPAQUE means "busy" and TRANSPARENT means
// "free" for availability calculations.
type EventOpacity string

const (
	OpacityOpaque      EventOpacity = "OPAQUE"
	OpacityTransparent EventOpacity = "TRANSPARENT"
)

// Attendee represents a VEVENT attendee or organizer.
type Attendee struct {
	Email  string              // raw e-mail address without "mailto:" prefix
	Name   string              // CN parameter
	RSVP   bool                // RSVP parameter
	Status ParticipationStatus // PARTSTAT parameter
}

// ParticipationStatus mirrors the iCal PARTSTAT parameter.
type ParticipationStatus string

const (
	PartStatNeedsAction ParticipationStatus = "NEEDS-ACTION"
	PartStatAccepted    ParticipationStatus = "ACCEPTED"
	PartStatDeclined    ParticipationStatus = "DECLINED"
	PartStatTentative   ParticipationStatus = "TENTATIVE"
)
