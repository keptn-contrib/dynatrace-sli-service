# from https://skaffold.dev/docs/workflows/debug/
# Use the offical Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.16.5 as builder

WORKDIR /go/src/github.com/keptn-contrib/dynatrace-sli-service

# Copy `go.mod` for definitions and `go.sum` to invalidate the next layer
# in case of a change in the dependencies
COPY go.mod go.sum ./

# download dependencies
RUN go mod download

# set buildflags for debug build
RUN if [ ! -z "$debugBuild" ]; then export BUILDFLAGS='-gcflags "all=-N -l"'; fi

# finally Copy local code to the container image.
COPY . .

# `skaffold debug` sets SKAFFOLD_GO_GCFLAGS to disable compiler optimizations
ARG SKAFFOLD_GO_GCFLAGS

# Build the command inside the container.
# (You may fetch or manage dependencies here, either manually or with a tool like "godep".)
RUN GOOS=linux go build -ldflags '-linkmode=external' -gcflags="${SKAFFOLD_GO_GCFLAGS}" -v -o dynatrace-sli-service ./cmd/

# Use a Docker multi-stage build to create a lean production image.
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
FROM alpine:3.14
ENV ENV=production

# Install extra packages
# See https://github.com/gliderlabs/docker-alpine/issues/136#issuecomment-272703023

RUN    apk update && apk upgrade \
	&& apk add ca-certificates libc6-compat \
	&& update-ca-certificates \
	&& rm -rf /var/cache/apk/*

# Copy the binary to the production image from the builder stage.
COPY --from=builder /go/src/github.com/keptn-contrib/dynatrace-sli-service/dynatrace-sli-service /dynatrace-sli-service

EXPOSE 8080

# required for external tools to detect this as a go binary
ENV GOTRACEBACK=all

# KEEP THE FOLLOWING LINES COMMENTED OUT!!! (they will be included within the CI build)
#build-uncomment ADD MANIFEST /
#build-uncomment COPY entrypoint.sh /
#build-uncomment ENTRYPOINT ["/entrypoint.sh"]

RUN adduser -D nonroot -u 65532
USER nonroot

CMD ["/dynatrace-sli-service"]
