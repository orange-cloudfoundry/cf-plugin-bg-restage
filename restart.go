package main

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/cli/cf/terminal"
	"github.com/contraband/autopilot/rewind"
)

func restartActions(appRepo *ApplicationRepo, appName string, cleanup cleanupAction) []rewind.Action {
	return []rewind.Action{
		// get droplet of existing app
		{
			Forward: func() error {
				appGUID, err := appRepo.GetAppGuid(appName)
				if err != nil {
					return err
				}
				return appRepo.DownloadDroplet(appGUID)
			},
		},
		// get manifest of existing app
		{
			Forward: func() error {
				return appRepo.CreateManifest(appName)
			},
		},
		// rename old app to app-venerable
		{
			Forward: func() error {
				return appRepo.RenameApplication(appName, venerableAppName(appName))
			},
			ReversePrevious: func() error {
				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// push new app with placeholder app bits
		{
			Forward: func() error {
				return appRepo.PushApplication(appName)
			},
			ReversePrevious: func() error {
				appRepo.DeleteApplication(appName)
				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// copy app bits from old app to new app
		{
			Forward: func() error {
				oldAppGUID, err := appRepo.GetAppGuid(venerableAppName(appName))
				if err != nil {
					return err
				}
				newAppGUID, err := appRepo.GetAppGuid(appName)
				if err != nil {
					return err
				}

				fmt.Printf("Copying application bits from %s to new %s\n",
					terminal.EntityNameColor(venerableAppName(appName)),
					terminal.EntityNameColor(appName),
				)
				pb := NewIndeterminateProgressBar(os.Stdout, "")

				job, err := appRepo.CopyBits(oldAppGUID, newAppGUID)
				if err != nil {
					return err
				}

				for {
					pb.Next()
					job, err := appRepo.GetJob(job.Entity.GUID)
					switch {
					case err != nil:
						fmt.Println("FAILED")
						return err
					case job.Entity.Status == "finished":
						fmt.Println("OK")
						return nil
					case job.Entity.Status == "failed":
						fmt.Println("FAILED")
						return fmt.Errorf(
							"Error %s, %s [code: %d]",
							job.Entity.ErrorDetails.ErrorCode,
							job.Entity.ErrorDetails.Description,
							job.Entity.ErrorDetails.Code,
						)
					}
					time.Sleep(500 * time.Millisecond)
				}
			},
			ReversePrevious: func() error {
				appRepo.DeleteApplication(appName)
				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// upload the droplet of the old app to the new app
		{
			Forward: func() error {
				return appRepo.UploadDroplet(appName)
			},
			ReversePrevious: func() error {
				appRepo.DeleteApplication(appName)
				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// start the new app
		{
			Forward: func() error {
				return appRepo.StartApplication(appName)
			},
			ReversePrevious: func() error {
				appRepo.DeleteApplication(appName)
				return appRepo.RenameApplication(venerableAppName(appName), appName)
			},
		},
		// cleanup the old app
		{
			Forward: func() error {
				switch cleanup {
				case deleteOnCleanup:
					return appRepo.DeleteApplication(venerableAppName(appName))
				case stopOnCleanup:
					return appRepo.StopApplication(venerableAppName(appName))
				default:
					return nil
				}
			},
		},
	}
}
