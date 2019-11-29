FROM alpine

RUN apk add --update jq

WORKDIR /radix-image-scanner/

COPY install_trivy.sh install_trivy.sh
RUN chmod +x /radix-image-scanner/install_trivy.sh

RUN sh /radix-image-scanner/install_trivy.sh

COPY scan_image.sh scan_image.sh
RUN chmod +x /radix-image-scanner/scan_image.sh

ENV TRIVY_AUTH_URL= \
    TRIVY_USERNAME= \
    TRIVY_PASSWORD=

CMD sh /radix-image-scanner/scan_image.sh