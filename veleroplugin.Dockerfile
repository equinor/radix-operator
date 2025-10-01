# Copyright 2017, 2019, 2020 the Velero contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM --platform=$BUILDPLATFORM docker.io/golang:1.24-alpine3.22 AS builder

ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

WORKDIR /src

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy and build project code
COPY . .
RUN go build -ldflags="-s -w" -o /build/radix-velero-plugin ./radix-velero-plugin

# Final stage
FROM docker.io/alpine:3
COPY --from=builder /build/radix-velero-plugin /plugins/
USER 65534
ENTRYPOINT ["/bin/sh", "-c", "cp /plugins/* /target/."]

