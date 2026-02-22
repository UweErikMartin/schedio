package config

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/coreos/pkg/flagutil"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

func readConfigFile(filePath string, args *Config) error {
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}
	err = yaml.Unmarshal(yamlFile, args)
	if err != nil {
		return fmt.Errorf("error unmarshalling config file: %v", err)
	}
	return nil
}

func normalizeRootPath(s string) string {
	cleaned := path.Clean("/" + s)
	if cleaned == "/" {
		return ""
	}
	return cleaned
}

func ParseCommandLineArgs(args []string) Config {
	var params Config

	// parse the command line arguments
	flags := flag.NewFlagSet(args[0], flag.ExitOnError)
	flags.StringVar(&params.Host, "host", "http://localhost", "The hostname of the server, used for the OpenAPI spec URL.")
	flags.IntVar(&params.Port, "port", 80, "The port to listen to for incoming HTTP requests.")
	flags.StringVar(&params.BindAddress, "bindAddress", "0.0.0.0", "The IP address on which to serve the --port (set to 0.0.0.0 for all interfaces).")
	flags.StringVar(&params.RootPath, "rootPath", "", "The root path the server is listening on")
	flags.StringVar(&params.SmtpUsername, "smtpUsername", "", "The username for sending mail2")
	flags.StringVar(&params.SmtpPassword, "smtpPassword", "", "The password for sending mails")
	flags.StringVar(&params.SmtpHost, "smtpHost", "", "The hostname of the outgoing smtp server")
	flags.IntVar(&params.SmtpPort, "smtpPort", 587, "The port of the outgoing smtp server")
	flags.StringVar(&params.AdminMail, "adminMail", "", "The email address of the admin")
	flags.StringVar(&params.MailTemplate, "mailTemplate", "", "The email address of the admin")
	flags.StringVar(&params.ConfigFile, "configFile", "", "read parameters from a yaml config file")
	flags.StringVar(&params.CalendarURL, "calendarUrl", "", "The URL of the CalDAV calendar")
	flags.StringVar(&params.CalendarUsername, "calendarUsername", "", "The username for the CalDAV calendar")
	flags.StringVar(&params.CalendarPassword, "calendarPassword", "", "The password for the CalDAV calendar")
	flags.IntVar(&params.Verbose, "verbose", 0, "Enable verbose logging. Set to 1 for info, 2 for debug, etc.")
	flags.BoolVar(&params.Dummy, "dummy", false, "Initialise the CalDAV backend with the built-in dummy store (read-only, pre-populated with sample events).")

	flags.Parse(args[1:])
	flagutil.SetFlagsFromEnv(flags, "SERVER")

	// Overwrite from config file
	if params.ConfigFile != "" {
		klog.Infof("read configuration from %s", params.ConfigFile)
		err := readConfigFile(params.ConfigFile, &params)
		if err != nil {
			klog.Fatalf("Error reading config file: %v", err)
		}
	}

	params.RootPath = normalizeRootPath(params.RootPath)

	return params
}
