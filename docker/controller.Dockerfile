ARG DOCKERHUB_REGISTRY=docker.io


FROM ${DOCKERHUB_REGISTRY}/library/golang:1.25.4 AS build

ARG VERSION=development

WORKDIR /usr/src
COPY go.mod /usr/src/go.mod
COPY go.sum /usr/src/go.sum

RUN go mod download

COPY cmd /usr/src/cmd
COPY internal /usr/src/internal

RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /usr/app/controller ./cmd/controller
FROM ${DOCKERHUB_REGISTRY}/library/alpine:3.23.0

WORKDIR /usr/app
RUN chown -R 1000:1000 /usr/app

COPY --from=build --chown=1000:1000 /usr/app/controller /usr/app/controller

USER 1000:1000
ENTRYPOINT ["/usr/app/controller"]