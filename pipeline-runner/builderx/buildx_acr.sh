#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

echo "Login to azure..."
az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT}

echo "Build with buildx (5)..."
az acr run --registry ${DOCKER_REGISTRY} -f build_with_cache.yaml \
    --set REGISTRY=${DOCKER_REGISTRY}.azurecr.io \
    --set IMAGE=${IMAGE} \
    --set PUSH=${PUSH} \
    --set BUILD_CONTEXT=${CONTEXT} \
    --set REPOSITORY_NAME=${REPOSITORY_NAME} \
    ./${DOCKER_FILE_NAME}