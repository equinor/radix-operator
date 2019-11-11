#!/bin/bash
function GetBuildCommand() {
  prefix="BUILD_SECRET_"
  delimiter='\='
  buildArgs=''
  buildCommand="az acr build -t ${IMAGE} -t ${CLUSTERTYPE_IMAGE} -t ${CLUSTERNAME_IMAGE} ${NO_PUSH} -r ${DOCKER_REGISTRY} ${CONTEXT} -f ${CONTEXT}${DOCKER_FILE_NAME}"

  while read -r line; do
      if [[ "$line" ]]; then
          keyValue=(${line//=/ })
          secretName=${keyValue[0]#"$prefix"}
          secretValue=${keyValue[1]}

          buildArgs+="--secret-build-arg $secretName=$secretValue "
      fi
  done <<< "$(env | grep 'BUILD_SECRET_')"

  buildCommand="$buildCommand $buildArgs"
  echo "$buildCommand"
}


if [[ -z "${SP_USER}" ]]; then
  SP_USER=$(cat ${AZURE_CREDENTIALS} | jq -r '.id')
fi

if [[ -z "${SP_SECRET}" ]]; then
  SP_SECRET=$(cat ${AZURE_CREDENTIALS} | jq -r '.password')
fi


azBuildCommand=$(GetBuildCommand)

az login --service-principal -u ${SP_USER} -p ${SP_SECRET} --tenant ${TENANT}
bash -c "$azBuildCommand"
