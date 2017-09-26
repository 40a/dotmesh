#!/bin/sh

cat << NIGHTWATCH_JSON > /home/node/nightwatch.json
{
  "globals_path": "globals.js",
  "output_folder": false,
  "src_folders": ["specs"],
  "page_objects_path": "lib/pages",
  "test_settings": {
    "default": {
      "screenshots": {
        "enabled": true,
        "on_failure": true,
        "path": "screenshots"
      },
      "videos": {
        "enabled": true,
        "delete_on_success": false,
        "path": "videos"
      },
      "launch_url": "http://${LAUNCH_URL}",
      "selenium_host": "${SELENIUM_HOST}",
      "desiredCapabilities" : {
        "browserName" : "chrome",
        "javascriptEnabled" : true,
        "acceptSslCerts" : true,
        "chromeOptions" : {
          "args" : ["disable-infobars"]
        }
      }
    }
  }
}
NIGHTWATCH_JSON

# shellcheck disable=SC2086
exec /sbin/tini -g -- \
  wait-for --timeout "${WAIT_FOR_TIMEOUT:-10}" $WAIT_FOR_HOSTS -- \
  nightwatch "$@"