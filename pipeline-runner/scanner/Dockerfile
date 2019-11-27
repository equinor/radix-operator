FROM microsoft/azure-cli:2.0.54
COPY install_trivy.sh install_trivy.sh
RUN chmod +x ./install_trivy.sh
RUN ./install_trivy.sh

WORKDIR /radix-image-scanner/
COPY scan_image.sh scan_image.sh

ENV TENANT=3aa4a235-b6e2-48d5-9195-7fcf05b459b0 \
    AZURE_CREDENTIALS=/radix-image-builder/.azure/sp_credentials.json \
    IMAGE_PATH=

RUN chmod +x /radix-image-scanner/scan_image.sh
ENTRYPOINT [ "/radix-image-scanner/scan_image.sh"]
CMD ["-c"]