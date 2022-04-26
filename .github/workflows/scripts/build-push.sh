#!/bin/bash

docker build . -f $DOCKERFILE -t $IMAGE_TAG
az acr login --name ${ACR_NAME}
docker push $IMAGE_TAG