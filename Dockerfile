################################################################################
# BUILDER/DEVELOPMENT IMAGE
################################################################################

FROM golang:1.13.8-alpine as builder

# Install Git
RUN apk add --no-cache git libc6-compat make

# go build will fail in alpine if this is enabled as it looks for gcc
ENV CGO_ENABLED 0

WORKDIR /build/

COPY go.mod /build/

RUN go mod download

# Copy all source code and required files into the build directory
COPY *.go /build/

# Build the executable
RUN go build -o promalert

################################################################################
# LINT IMAGE
################################################################################

FROM golang:1.13.8 as ci

# Install golangci
RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.21.0

WORKDIR /app

COPY --from=builder /build .

RUN go mod download

COPY .golangci.yml .

################################################################################
# FINAL IMAGE
################################################################################

FROM alpine:3.11

LABEL com.bugsnag.app="promalert"

COPY --from=builder /build/promalert /app/
COPY config.example.yaml /app/config.yml

CMD [ "/app/promalert" ]