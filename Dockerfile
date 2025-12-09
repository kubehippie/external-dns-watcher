FROM --platform=$BUILDPLATFORM golang:1.25@sha256:0ece421d4bb2525b7c0b4cad5791d52be38edf4807582407525ca353a429eccc AS builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum

RUN --mount=type=cache,target=/go/pkg \
    go mod download -x

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

FROM gcr.io/distroless/static:nonroot@sha256:2b7c93f6d6648c11f0e80a48558c8f77885eb0445213b8e69a6a0d7c89fc6ae4
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
