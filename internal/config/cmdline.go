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
	flags.StringVar(&params.SenderName, "smtpSenderName", "Schedio Buchungssystem", "The display name used in the From: header of all outgoing e-mails")
	flags.StringVar(&params.AdminMail, "adminMail", "", "The email address of the admin")
	flags.StringVar(&params.MailTemplate, "mailTemplate", "", "The email address of the admin")
	flags.StringVar(&params.ConfigFile, "configFile", "", "read parameters from a yaml config file")
	flags.StringVar(&params.CalendarURL, "calendarUrl", "", "The URL of the CalDAV calendar")
	flags.StringVar(&params.CalendarUsername, "calendarUsername", "", "The username for the CalDAV calendar")
	flags.StringVar(&params.CalendarPassword, "calendarPassword", "", "The password for the CalDAV calendar")
	flags.IntVar(&params.Verbose, "verbose", 0, "Enable verbose logging. Set to 1 for info, 2 for debug, etc.")
	flags.BoolVar(&params.Dummy, "dummy", false, "Initialise the CalDAV backend with the built-in dummy store (read-only, pre-populated with sample events).")
	flags.StringVar(&params.ServicesFile, "servicesFile", "", "Path to a YAML file containing the list of bookable services (see config/services.yaml for the format).")
	flags.StringVar(&params.UsersFile, "usersFile", "", "Path to a YAML file containing admin/staff user accounts (see config/users.yaml for the format).")
	flags.StringVar(&params.AvailabilityFile, "availabilityFile", "", "Path to a YAML file containing staff timeslots for testing (see config/availability.yaml for the format).")

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

	if params.ServicesFile != "" {
		klog.Infof("reading services from %s", params.ServicesFile)
		if err := readServicesFile(params.ServicesFile, &params); err != nil {
			klog.Fatalf("error reading services file: %v", err)
		}
	} else {
		klog.V(2).Info("no -servicesFile given, using built-in dummy services")
		params.Services = defaultServices
	}

	if params.UsersFile != "" {
		klog.Infof("reading users from %s", params.UsersFile)
		if err := readUsersFile(params.UsersFile, &params); err != nil {
			klog.Fatalf("error reading users file: %v", err)
		}
	} else {
		klog.V(2).Info("no -usersFile given, no users pre-loaded")
	}

	if params.AvailabilityFile != "" {
		klog.Infof("reading timeslots from %s", params.AvailabilityFile)
		if err := readAvailabilityFile(params.AvailabilityFile, &params); err != nil {
			klog.Fatalf("error reading availability file: %v", err)
		}
	} else {
		klog.V(2).Info("no -availabilityFile given, no timeslots pre-loaded")
	}

	return params
}

func readUsersFile(filePath string, args *Config) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}
	var uf usersFile
	if err := yaml.Unmarshal(data, &uf); err != nil {
		return fmt.Errorf("parse %s: %w", filePath, err)
	}
	args.Users = uf.Users
	return nil
}

func readServicesFile(filePath string, args *Config) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}
	var sf servicesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("parse %s: %w", filePath, err)
	}
	args.Services = sf.Services
	return nil
}

func readAvailabilityFile(filePath string, args *Config) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}
	var af availabilityFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return fmt.Errorf("parse %s: %w", filePath, err)
	}
	args.Timeslots = af.Timeslots
	return nil
}
