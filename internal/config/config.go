package config

type Config struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	BindAddress      string `yaml:"bindAddress"`
	RootPath         string `yaml:"rootPath"`
	SmtpUsername     string `yaml:"smtpUsername"`
	SmtpPassword     string `yaml:"smtpPassword"`
	SmtpHost         string `yaml:"smtpHost"`
	SmtpPort         int    `yaml:"smtpPort"`
	AdminMail        string `yaml:"adminMail"`
	MailTemplate     string `yaml:"mailTemplate"`
	CalendarURL      string `yaml:"calendarUrl"`
	CalendarUsername string `yaml:"calendarUsername"`
	CalendarPassword string `yaml:"calendarPassword"`
	Verbose          int    `yaml:"verbose"`
	Dummy            bool   `yaml:"dummy"`
	ConfigFile       string `yaml:"-"`
}
