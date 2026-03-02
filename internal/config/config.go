package config

// defaultServices is used when no -servicesFile flag is supplied.
var defaultServices = []ServiceEntry{
	{
		ID:              "d0000000-0000-4000-8000-000000000001",
		Name:            "Beratungsgespräch",
		Summary:         "Erstgespräch und Beratung.",
		Description:     "Ein ausführliches Erstgespräch zur Klärung Ihrer Anliegen und zur Vorstellung unserer Behandlungsangebote.",
		Price:           0,
		DurationMinutes: 30,
		DailyLimit:      0,
	},
	{
		ID:              "d0000000-0000-4000-8000-000000000002",
		Name:            "Standardbehandlung",
		Summary:         "Unsere klassische Einzelbehandlung.",
		Description:     "Eine 60-minütige Einzelbehandlung nach individueller Absprache.",
		Price:           80.00,
		DurationMinutes: 60,
		DailyLimit:      4,
	},
}

// ServiceEntry describes a single bookable service as parsed from the services
// config file (see -servicesFile flag).
type ServiceEntry struct {
	ID              string  `yaml:"uid"`
	Name            string  `yaml:"name"`
	Summary         string  `yaml:"summary"`
	Description     string  `yaml:"description"`
	Price           float64 `yaml:"price"`
	DurationMinutes int     `yaml:"duration_minutes"`
	DailyLimit      int     `yaml:"daily_limit"`
}

// servicesFile is the on-disk envelope that wraps the list of services.
type servicesFile struct {
	Services []ServiceEntry `yaml:"services"`
}

// UserEntry describes a single user as parsed from the users config file
// (see -usersFile flag and config/users.yaml for the format).
type UserEntry struct {
	ID                string `yaml:"id,omitempty"` // stable UUID; generated at startup when absent
	Email             string `yaml:"email"`
	PasswordHash      string `yaml:"password_hash"`
	Role              string `yaml:"role"`
	AppleOAuthEnabled bool   `yaml:"apple_oauth_enabled"`
	AppleSubject      string `yaml:"apple_subject"`
	Name              string `yaml:"name"`
}

// usersFile is the on-disk envelope that wraps the list of users.
type usersFile struct {
	Users []UserEntry `yaml:"users"`
}

// TimeslotEntry describes a single staff availability window as parsed from
// the availability config file (see -availabilityFile flag and
// config/availability.yaml for the format).
type TimeslotEntry struct {
	UID     string `yaml:"uid"`             // stable CalDAV UID; used as idempotency key
	UserID  string `yaml:"user_id"`         // UUID of the owning staff user (must match UserEntry.ID)
	StartAt string `yaml:"start_at"`        // RFC 3339 / ISO 8601 datetime (UTC)
	EndAt   string `yaml:"end_at"`          // RFC 3339 / ISO 8601 datetime (UTC)
	RRule   string `yaml:"rrule,omitempty"` // iCal RRULE value; empty for single events
}

// availabilityFile is the on-disk envelope that wraps the list of timeslots.
type availabilityFile struct {
	Timeslots []TimeslotEntry `yaml:"timeslots"`
}

// Config holds all runtime configuration derived from flags, environment
// variables, and optional config files.
type Config struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	BindAddress      string `yaml:"bindAddress"`
	RootPath         string `yaml:"rootPath"`
	SmtpUsername     string `yaml:"smtpUsername"`
	SmtpPassword     string `yaml:"smtpPassword"`
	SmtpHost         string `yaml:"smtpHost"`
	SmtpPort         int    `yaml:"smtpPort"`
	SenderName       string `yaml:"smtpSenderName"`
	AdminMail        string `yaml:"adminMail"`
	MailTemplate     string `yaml:"mailTemplate"`
	CalendarURL      string `yaml:"calendarUrl"`
	CalendarUsername string `yaml:"calendarUsername"`
	CalendarPassword string `yaml:"calendarPassword"`
	Verbose          int    `yaml:"verbose"`
	Dummy            bool   `yaml:"dummy"`
	// NoAuth disables all HTTP Basic Auth checks on CalDAV endpoints.
	// Intended for local development only when the client (e.g. iOS) refuses
	// to send credentials over plain HTTP.  Never enable in production.
	NoAuth           bool            `yaml:"noAuth"`
	ConfigFile       string          `yaml:"-"`
	ServicesFile     string          `yaml:"servicesFile"`
	Services         []ServiceEntry  `yaml:"-"`
	UsersFile        string          `yaml:"usersFile"`
	Users            []UserEntry     `yaml:"-"`
	AvailabilityFile string          `yaml:"availabilityFile"`
	Timeslots        []TimeslotEntry `yaml:"-"`
}
