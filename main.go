package main

import (
	"code.cloudfoundry.org/cli/cf/terminal"
	"code.cloudfoundry.org/cli/plugin"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/contraband/autopilot/rewind"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BgRestagePlugin struct{}
type Job struct {
	Metadata struct {
		GUID      string    `json:"guid"`
		CreatedAt time.Time `json:"created_at"`
		URL       string    `json:"url"`
	} `json:"metadata"`
	Entity struct {
		GUID         string `json:"guid"`
		Status       string `json:"status"`
		Error        string `json:"error"`
		ErrorDetails struct {
			Code        int    `json:"code"`
			Description string `json:"description"`
			ErrorCode   string `json:"error_code"`
		} `json:"error_details"`
	} `json:"entity"`
}

func venerableAppName(appName string) string {
	return fmt.Sprintf("%s-venerable", appName)
}
func restageActions(appRepo *ApplicationRepo, appName string) []rewind.Action {
	return []rewind.Action{
		// create manifest
		{
			Forward: func() error {
				return appRepo.CreateManifest(appName)
			},
		},
		// create fake file to deploy
		{
			Forward: func() error {
				return appRepo.TouchDir()
			},
		},
		// rename
		{
			Forward: func() error {
				return appRepo.RenameApplication(appName, venerableAppName(appName))
			},
		},
		// push
		{
			Forward: func() error {
				appRepo.PushApplication(appName)
				return nil
			},
		},
		// Copy bits
		{
			Forward: func() error {
				oldAppGuid, err := appRepo.GetAppGuid(venerableAppName(appName))
				if err != nil {
					return err
				}
				newAppGuid, err := appRepo.GetAppGuid(appName)
				if err != nil {
					return err
				}
				job, err := appRepo.CopyBits(oldAppGuid, newAppGuid)
				if err != nil {
					return err
				}
				pb := NewIndeterminateProgressBar(
					os.Stdout,
					fmt.Sprintf("copying bits from %s to new %s",
						terminal.EntityNameColor(venerableAppName(appName)),
						terminal.EntityNameColor(appName),
					),
				)
				for {
					job, err := appRepo.GetJob(job.Entity.GUID)
					if err != nil {
						return err
					}
					if job.Entity.Status == "finished" {
						return nil
					}
					if job.Entity.Status == "failed" {
						return fmt.Errorf(
							"Error %s, %s [code: %d]",
							job.Entity.ErrorDetails.ErrorCode,
							job.Entity.ErrorDetails.Description,
							job.Entity.ErrorDetails.Code,
						)
					}
					pb.Next()
				}
				return nil
			},
			ReversePrevious: func() error {
				// If the app cannot start we'll have a lingering application
				// We delete this application so that the rename can succeed
				appRepo.DeleteApplication(appName)

				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// restart
		{
			Forward: func() error {
				fmt.Println()
				return appRepo.RestartApplication(appName)
			},
			ReversePrevious: func() error {
				// If the app cannot start we'll have a lingering application
				// We delete this application so that the rename can succeed
				appRepo.DeleteApplication(appName)

				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// delete
		{
			Forward: func() error {
				return appRepo.DeleteApplication(venerableAppName(appName))
			},
		},
	}
}
func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stdout, "error:", err)
		os.Exit(1)
	}
}
func main() {
	plugin.Start(&BgRestagePlugin{})
}
func (plugin BgRestagePlugin) Run(cliConnection plugin.CliConnection, args []string) {
	appRepo, err := NewApplicationRepo(cliConnection)
	fatalIf(err)
	defer appRepo.DeleteDir()
	if len(args) < 2 {
		fatalIf(fmt.Errorf("Usage: cf bg-restage application-to-restage"))
	}
	appName := args[1]
	actionList := restageActions(appRepo, appName)
	actions := rewind.Actions{
		Actions:              actionList,
		RewindFailureMessage: "Oh no. Something's gone wrong. I've tried to roll back but you should check to see if everything is OK.",
	}
	err = actions.Execute()
	fatalIf(err)

	fmt.Println()
	fmt.Println("Your application has been restaged with no downtime !")
	fmt.Println()

	_ = appRepo.ListApplications()
}

func (BgRestagePlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "bg-restage",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "bg-restage",
				HelpText: "Perform a zero-downtime restage of an application over the top of an old one",
				UsageDetails: plugin.Usage{
					Usage: "$ cf bg-restage application-to-restage",
				},
			},
		},
	}
}

type ApplicationRepo struct {
	conn plugin.CliConnection
	dir  string
}

func NewApplicationRepo(conn plugin.CliConnection) (*ApplicationRepo, error) {
	dir, err := ioutil.TempDir("", "bg-restage")
	if err != nil {
		return nil, err
	}
	return &ApplicationRepo{
		conn: conn,
		dir:  dir,
	}, nil
}
func (repo *ApplicationRepo) DeleteDir() error {
	return os.RemoveAll(repo.dir)
}

func (repo *ApplicationRepo) CreateManifest(name string) error {
	_, err := repo.conn.CliCommand("create-app-manifest", name, "-p", repo.manifestFilePath())
	return err
}
func (repo *ApplicationRepo) manifestFilePath() string {
	return filepath.Join(repo.dir, "manifest.yml")
}
func (repo *ApplicationRepo) TouchDir() error {
	f, err := os.Create(repo.dir + "/nofile")
	if err != nil {
		return err
	}

	defer f.Close()
	return nil
}

func (repo *ApplicationRepo) RenameApplication(oldName, newName string) error {
	_, err := repo.conn.CliCommand("rename", oldName, newName)
	return err
}

func (repo *ApplicationRepo) PushApplication(appName string) error {
	args := []string{"push", appName, "-f", repo.manifestFilePath(), "-p", repo.dir, "--no-start"}
	_, err := repo.conn.CliCommand(args...)
	return err
}
func (repo *ApplicationRepo) RestartApplication(appName string) error {
	args := []string{"restart", appName}
	_, err := repo.conn.CliCommand(args...)
	return err
}
func (repo *ApplicationRepo) DeleteApplication(appName string) error {
	_, err := repo.conn.CliCommand("delete", appName, "-f")
	return err
}

func (repo *ApplicationRepo) ListApplications() error {
	_, err := repo.conn.CliCommand("apps")
	return err
}

func (repo *ApplicationRepo) CopyBits(oldAppGuid, newAppGuid string) (Job, error) {
	respSlice, err := repo.conn.CliCommandWithoutTerminalOutput(
		"curl",
		"-X",
		"POST",
		fmt.Sprintf("/v2/apps/%s/copy_bits", newAppGuid),
		"-d",
		fmt.Sprintf(`{"source_app_guid":"%s"}`, oldAppGuid),
	)
	if err != nil {
		return Job{}, err
	}
	resp := strings.Join(respSlice, "\n")
	var job Job
	err = json.Unmarshal([]byte(resp), &job)
	if err != nil {
		return Job{}, err
	}
	return job, nil
}
func (repo *ApplicationRepo) GetJob(jobGuid string) (Job, error) {
	respSlice, err := repo.conn.CliCommandWithoutTerminalOutput(
		"curl",
		fmt.Sprintf("/v2/jobs/%s", jobGuid),
	)
	resp := strings.Join(respSlice, "\n")
	var job Job
	err = json.Unmarshal([]byte(resp), &job)
	if err != nil {
		return Job{}, err
	}
	return job, nil
}
func (repo *ApplicationRepo) GetAppGuid(name string) (string, error) {
	d, err := repo.conn.CliCommandWithoutTerminalOutput("app", name, "--guid")
	if err != nil {
		return "", err
	}
	if len(d) == 0 {
		return "", fmt.Errorf("app '%s' not found.", name)
	}
	return d[0], err
}

func (repo *ApplicationRepo) DoesAppExist(appName string) (bool, error) {
	space, err := repo.conn.GetCurrentSpace()
	if err != nil {
		return false, err
	}

	path := fmt.Sprintf(`v2/apps?q=name:%s&q=space_guid:%s`, url.QueryEscape(appName), space.Guid)
	result, err := repo.conn.CliCommandWithoutTerminalOutput("curl", path)

	if err != nil {
		return false, err
	}

	jsonResp := strings.Join(result, "")

	output := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonResp), &output)

	if err != nil {
		return false, err
	}

	totalResults, ok := output["total_results"]

	if !ok {
		return false, errors.New("Missing total_results from api response")
	}

	count, ok := totalResults.(float64)

	if !ok {
		return false, fmt.Errorf("total_results didn't have a number %v", totalResults)
	}

	return count == 1, nil
}
