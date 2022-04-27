#!/bin/bash

sha=${GITHUB_SHA::8}
ts=$(date +%s)
build_id=${GITHUB_REF_NAME}-${sha}-${ts}

image_tag=${ACR_NAME}.azurecr.io/${OPERATOR_IMAGE_NAME}:$build_id
docker build . -f operator.Dockerfile -t $image_tag

az acr login --name ${ACR_NAME}
docker push $image_tag