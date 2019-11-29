#!/bin/bash
if test -f "${AZURE_CREDENTIALS}"; then
  echo "Authenticating with ACR"

  if [[ -z "${TRIVY_USERNAME}" ]]; then
    TRIVY_USERNAME=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
  fi

  if [[ -z "${TRIVY_PASSWORD}" ]]; then
    TRIVY_PASSWORD=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
  fi
fi

trivy -q --timeout 20m --severity HIGH,CRITICAL ${IMAGE_PATH}
exit 0
