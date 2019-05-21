#!/bin/bash

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.3.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/base.html and update docVersions variable
PORT=6600

cd $(dirname $0)
./build.sh || exit $?

trap "exit" INT TERM ERR
trap "kill 0" EXIT

sass -C --precision 9 --sourcemap=none --watch theme/src/index.scss:theme/css/teleport-bundle.css &

echo -e "\n\n----> LIVE EDIT HERE: http://localhost:$PORT/admin-guide/\n"
mkdocs serve --livereload --config-file=latest.yaml --dev-addr=0.0.0.0:$PORT & wait
