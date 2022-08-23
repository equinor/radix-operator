#!/bin/bash

sha=${GITHUB_SHA::8}
ts=$(date +%s)
build_id=${GITHUB_REF_NAME}-${sha}-${ts}

image_tag=${ACR_NAME}.azurecr.io/${OPERATOR_IMAGE_NAME}:$build_id
az acr task run \
    --subscription ${AZURE_SUBSCRIPTION_ID} \
    --name radix-image-builder-internal \
    --registry ${ACR_NAME} \
    --context ${GITHUB_WORKSPACE} \
    --file ${GITHUB_WORKSPACE}/Dockerfile \
    --set DOCKER_REGISTRY=${ACR_NAME} \
    --set BRANCH=${GITHUB_REF_NAME} \
    --set TAGS="--tag ${image_tag}" \
    --set DOCKER_FILE_NAME=Dockerfile \
    --set PUSH="--push" \
    --set REPOSITORY_NAME=${IMAGE_NAME} \
    --set CACHE="" \
    --set CACHE_TO_OPTIONS="--cache-to=type=registry,ref=${ACR_NAME}.azurecr.io/${IMAGE_NAME}:radix-cache-${GITHUB_REF_NAME},mode=max"