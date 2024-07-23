FROM --platform=$BUILDPLATFORM golang:1.22 AS build-stage

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o /pc321-to-ha

FROM --platform=$BUILDPLATFORM gcr.io/distroless/base-debian12 AS build-release-stage
ARG TARGETOS TARGETARCH
WORKDIR /
COPY --from=build-stage /pc321-to-ha /pc321-to-ha
USER nonroot:nonroot
ENTRYPOINT ["/pc321-to-ha"]
