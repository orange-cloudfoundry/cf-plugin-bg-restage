#!/usr/bin/env bash
CURRENTDIR=`pwd`
PLUGIN_PATH="$CURRENTDIR/out/cf-plugin-bg-restage"

$CURRENTDIR/bin/build
cf uninstall-plugin bg-restage
cf install-plugin "$PLUGIN_PATH" -f