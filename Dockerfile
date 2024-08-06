################################################################################
# BUILDER/DEVELOPMENT IMAGE
################################################################################

FROM golang:1.22.5-alpine as builder

# Install Git
RUN apk add --no-cache git libc6-compat make

# Enable go modules so we can download go tools with specific versions
ENV GO111MODULE=on

# go build will fail in alpine if this is enabled as it looks for gcc
ENV CGO_ENABLED 0

# Copy all source code and required files into the build directory
COPY . /build/
WORKDIR /build/

# Build the executable
RUN go build -o promalert

################################################################################
# LINT IMAGE
################################################################################

FROM golang:1.22.5 as ci

# Install golangci
RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.22.5

WORKDIR /app

COPY --from=builder /build .

RUN go mod download

################################################################################
# FINAL IMAGE
################################################################################

FROM alpine:3.20

LABEL com.bugsnag.app="promalert"

COPY --from=builder /build/promalert /app/
COPY config.bugsnag.yaml /app/config.yml

CMD [ "/app/promalert" ]
