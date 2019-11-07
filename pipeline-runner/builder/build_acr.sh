#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

env | grep BUILD_SECRET_

az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT} 
az acr build -t ${IMAGE} ${NO_PUSH} -r ${DOCKER_REGISTRY} ${CONTEXT} -f ${CONTEXT}${DOCKER_FILE_NAME}