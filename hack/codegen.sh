#!/bin/bash
ROOT_PACKAGE="github.com/statoil/radix"
CUSTOM_RESOURCE_NAME="radix"
CUSTOM_RESOURCE_VERSION="v1"
./generate-groups.sh all "$ROOT_PACKAGE/pkg/client" "$ROOT_PACKAGE/pkg/apis" "$CUSTOM_RESOURCE_NAME:$CUSTOM_RESOURCE_VERSION"