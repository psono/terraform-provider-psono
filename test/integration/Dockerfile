# syntax=docker/dockerfile:1@sha256:ecfaec9ed6d810b56388c508f4121597bfbba70d41a6dfeee4d8cad5f295fc32

FROM golang:1.26.7@sha256:45a5f7a810238aabcbad211d70b9ae082022d96f7c7259e94041ad1b933575ac AS build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY test/integration/verify ./test/integration/verify
RUN mkdir -p "/out/plugins/registry.terraform.io/psono/psono/0.0.1/linux_${TARGETARCH}" \
    && CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=0.0.1' \
       -o "/out/plugins/registry.terraform.io/psono/psono/0.0.1/linux_${TARGETARCH}/terraform-provider-psono_v0.0.1" \
       ./cmd/terraform-provider-psono \
    && CGO_ENABLED=0 go build -trimpath -o /out/psono-integration-verify ./test/integration/verify

FROM hashicorp/terraform:1.15.9@sha256:fd5debae63188975d6febc6aa5bd1a982a588f55e4a4ddb7de28be923f250456
COPY --from=build /out/plugins /plugins
COPY --from=build /out/psono-integration-verify /usr/local/bin/psono-integration-verify
COPY test/integration/main.tf test/integration/terraform.rc test/integration/run.sh /test/
WORKDIR /work
ENTRYPOINT ["/bin/sh", "/test/run.sh"]
