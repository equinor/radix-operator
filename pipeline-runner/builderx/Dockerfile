FROM microsoft/azure-cli:2.0.54
WORKDIR /radix-image-builderx/
COPY build_with_cache.yaml build_with_cache.yaml
COPY buildx_acr.sh buildx_acr.sh

ENV TENANT=3aa4a235-b6e2-48d5-9195-7fcf05b459b0 \
    AZURE_CREDENTIALS=/radix-image-builder/.azure/sp_credentials.json \
    DOCKER_REGISTRY=radixdev \
    DOCKER_FILE_NAME=Dockerfile \
    CONTEXT=./workspace/ \
    PUSH=--push

RUN chmod +r /radix-image-builderx/build_with_cache.yaml
RUN chmod +x /radix-image-builderx/buildx_acr.sh
ENTRYPOINT [ "/radix-image-builderx/buildx_acr.sh"]
CMD ["-c"]