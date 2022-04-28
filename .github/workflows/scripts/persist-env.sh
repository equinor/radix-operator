#!/bin/sh
config_name=$1
env -i GITHUB_WORKSPACE=$GITHUB_WORKSPACE /bin/bash -c "set -a && source $GITHUB_WORKSPACE/.github/workflows/config/${GITHUB_REF_NAME}/${config_name} && printenv" > /tmp/env_vars
while read -r env_var
do
    echo "$env_var" >> $GITHUB_ENV
done < /tmp/env_vars