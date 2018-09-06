package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/pkg/errors"
)

const (
	dropletFileName  = "droplet"
	manifestFileName = "manifest.yml"
)

type ApplicationRepo struct {
	conn plugin.CliConnection
	dir  string
}

func NewApplicationRepo(conn plugin.CliConnection) (*ApplicationRepo, error) {
	dir, err := ioutil.TempDir("", "bg-restage-plugin")
	if err != nil {
		return nil, errors.Wrap(err, "creating temporary directory")
	}
	err = ioutil.WriteFile(filepath.Join(dir, ".cfignore"), ([]byte)(dropletFileName+"\n"), 0600)
	if err != nil {
		return nil, errors.Wrap(err, "creating .cfignore")
	}
	f, err := os.Create(filepath.Join(dir, ".app_bits_placeholder"))
	if err == nil {
		defer f.Close()
	}

	return &ApplicationRepo{
		conn: conn,
		dir:  dir,
	}, nil
}

func (repo *ApplicationRepo) DeleteDir() error {
	return os.RemoveAll(repo.dir)
}

func (repo *ApplicationRepo) CreateManifest(appName string) error {
	// FIXME: this function should use the appGUID instead
	_, err := repo.conn.CliCommand("create-app-manifest", appName, "-p", repo.manifestFilePath())
	return err
}

func (repo *ApplicationRepo) manifestFilePath() string {
	return filepath.Join(repo.dir, manifestFileName)
}

func (repo *ApplicationRepo) RenameApplication(oldName, newName string) error {
	// FIXME: this function should use the old appGUID instead
	_, err := repo.conn.CliCommand("rename", oldName, newName)
	return err
}

func (repo *ApplicationRepo) PushApplication(appName string) error {
	_, err := repo.conn.CliCommand("push", appName, "-f", repo.manifestFilePath(), "-p", repo.dir, "--no-start")
	return err
}

func (repo *ApplicationRepo) DownloadDroplet(appGUID string) error {
	_, err := repo.conn.CliCommandWithoutTerminalOutput(
		"curl",
		fmt.Sprintf("/v2/apps/%s/droplet/download", appGUID),
		"--output",
		repo.dropletFilePath(),
	)
	return err
}

func (repo *ApplicationRepo) UploadDroplet(appName string) error {
	// FIXME: this unfortunately overrides/resets the buildpack and stack
	// setting on the cloud controller, even if they were set correctly via
	// the manifest
	// FIXME: this function should use the appGUID instead
	// FIXME: the following line is broken due to https://github.com/cloudfoundry/cli/issues/1445 so we shell out instead
	// _, err := repo.conn.CliCommand("push", appName, "--no-manifest", "--droplet", repo.dropletFilePath(), "--no-start")
	fmt.Println("Uploading droplet")
	err := exec.Command("cf", "push", appName, "--no-manifest", "--droplet", repo.dropletFilePath(), "--no-start").Run()
	if err == nil {
		fmt.Println("OK")
	} else {
		fmt.Println("FAILED")
	}
	return err
}

func (repo *ApplicationRepo) dropletFilePath() string {
	return filepath.Join(repo.dir, dropletFileName)
}

func (repo *ApplicationRepo) StartApplication(appName string) error {
	// FIXME: this function should use the appGUID instead
	_, err := repo.conn.CliCommand("start", appName)
	return err
}

func (repo *ApplicationRepo) StopApplication(appName string) error {
	// FIXME: this function should use the appGUID instead
	_, err := repo.conn.CliCommand("stop", appName)
	return err
}

func (repo *ApplicationRepo) DeleteApplication(appName string) error {
	// FIXME: this function should use the appGUID instead
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
	d, err := repo.conn.CliCommandWithoutTerminalOutput("app", appName, "--guid")
	if err != nil {
		return false, err
	}
	return len(d) > 0, nil
}

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
