#!/bin/bash
if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi

az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT}
docker pull ${IMAGE_PATH}
trivy --severity HIGH,CRITICAL ${IMAGE_PATH}
