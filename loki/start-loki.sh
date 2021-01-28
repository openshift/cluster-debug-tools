#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

# Starts loki in a docker container with customizable configuration
# and persistent data path.
#
# By default the config template will be symlinked as the config file.
# To override the default configuration:
#
#  $ cp -f config/loki-config.yaml.template config/loki-config.yaml

LOKI_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
DATA_PATH="${LOKI_ROOT}/data"
CONFIG_PATH="${LOKI_ROOT}/config"

if [[ -d "${DATA_PATH}" ]]; then
  mkdir -p "${DATA_PATH}"
  # Ensure container can write to the path
  chmod 777 "${DATA_PATH}"
fi

# Use the template config by default
if [[ ! -f "${CONFIG_PATH}/loki-config.yaml" ]]; then
  # Use a relative symlink to ensure it will work even for a different
  # absolute path (like in the container).
  pushd "${CONFIG_PATH}" > /dev/null
    ln -s loki-config.yaml.template loki-config.yaml
  popd > /dev/null
fi

docker run -d -v ${CONFIG_PATH}:/loki-config -v ${DATA_PATH}:/loki\
       -p 3100:3100 --name=loki grafana/loki:2.0.0\
       -config.file=/loki-config/loki-config.yaml
