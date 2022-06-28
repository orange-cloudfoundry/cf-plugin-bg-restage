package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/contraband/autopilot/rewind"
)

var (
	Major string
	Minor string
	Patch string
)

type BgRestagePlugin struct{}

func (p BgRestagePlugin) Run(cliConnection plugin.CliConnection, args []string) {
	// wrap the logic in run() so that we can use defer for cleanup there
	// (defer doesn't work with os.Exit())
	if err := p.run(cliConnection, args); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

func (BgRestagePlugin) run(cliConnection plugin.CliConnection, args []string) error {
	action := args[0]
	if action == "CLI-MESSAGE-UNINSTALL" {
		return nil
	}

	fs := flag.NewFlagSet("cf "+action, flag.ExitOnError)
	nodelete := fs.Bool("no-delete", false, "Stop but do not delete the old copy of the application when "+action+" completes")
	nostop := fs.Bool("no-stop", false, "Do not stop the old copy of the application when "+action+" completes (implies --no-delete)")
	venerable := fs.String("venerable-suffix", "-venerable", "Suffix appended to the name of the old copy of the application")
	fs.Parse(args[1:])
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("no application name specified")
	}

	appName := fs.Arg(0)
	cleanup := deleteOnCleanup
	if *nostop { // nostop takes precedence over nodelete (it's implicit that you can't delete without stopping)
		cleanup = skipCleanup
	} else if *nodelete {
		cleanup = stopOnCleanup
	}

	if *venerable == "" {
		fs.Usage()
		return fmt.Errorf("illegal --venerable-suffix")
	}
	venerableSuffix = *venerable

	appRepo, err := NewApplicationRepo(cliConnection)
	if err != nil {
		return err
	}
	defer appRepo.DeleteDir()

	var actionList []rewind.Action
	if action == "bg-restage" {
		actionList = restageActions(appRepo, appName, cleanup)
	} else /* action == "bg-restart" */ {
		actionList = restartActions(appRepo, appName, cleanup)
	}
	actions := rewind.Actions{
		Actions:              actionList,
		RewindFailureMessage: action + " failed: an attempt was made at rolling back changes. Please verify that everything is fine.",
	}

	if err := actions.Execute(); err != nil {
		return err
	}

	fmt.Print("\n" + action + " completed successfully\n\n")

	_ = appRepo.ListApplications()

	return nil
}

func (BgRestagePlugin) GetMetadata() plugin.PluginMetadata {
	major := 0
	minor := 0
	patch := 0
	major, _ = strconv.Atoi(Major)
	minor, _ = strconv.Atoi(Minor)
	patch, _ = strconv.Atoi(Patch)

	return plugin.PluginMetadata{
		Name: "bg-restage",
		Version: plugin.VersionType{
			Major: major,
			Minor: minor,
			Build: patch,
		},
		Commands: []plugin.Command{
			{
				Name:     "bg-restage",
				HelpText: "Perform a zero-downtime restage of an application",
				UsageDetails: plugin.Usage{
					Usage: "$ cf bg-restage application-to-restage",
				},
			},
			{
				Name:     "bg-restart",
				HelpText: "Perform a zero-downtime restart of an application",
				UsageDetails: plugin.Usage{
					Usage: "$ cf bg-restart application-to-restage",
				},
			},
		},
	}
}

var venerableSuffix string // FIXME: this should not be a global variable

func venerableAppName(appName string) string {
	return fmt.Sprintf("%s%s", appName, venerableSuffix)
}

type cleanupAction int

const (
	skipCleanup cleanupAction = iota
	stopOnCleanup
	deleteOnCleanup
)

func main() {
	plugin.Start(&BgRestagePlugin{})
}
