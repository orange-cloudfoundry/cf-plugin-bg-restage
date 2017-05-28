# bg-restage

cf plugin for zero downtime application restaging, highly inspired by [autopilot](https://github.com/contraband/autopilot).

This is mostly for those who don't want to redeploy an app with source code. 
It was initially created to restage automatically all apps in a Cloud Foundry 
when a new buildpack as been released with security patch.

## Installation

Download the latest version from the [releases][releases] page and make it executable.

```
$ cf install-plugin path/to/downloaded/binary
```

[releases]: https://github.com/orange-cloudfoundry/cf-plugin-bg-restage/releases

## Usage

```
$ cf bg-restage application-to-restage
```

## Method

1. It retrieves manifest from old app in a directory and create fake file as content to be pushed.

2. The old application is renamed to `<APP-NAME>-venerable`. It keeps its old route
   mappings and this change is invisible to users.

3. The new application is pushed to `<APP-NAME>`, this push will normally failed because we just want to create an app
   but not push real code (we do that because there is no easy way to create an app without pushing code as a cli plugin). 
   **Note**: you will not see any failures and if it's not failed the app will not be started.

4. Bits will be copied from old app to the new app to put real code inside the new app.

5. The new app will be restarted which will restage the app with the real code from old app.

6. The old app will be removed and all traffic will be on the new app.
