FROM --platform=$BUILDPLATFORM golang:1.26@sha256:9edf71320ef8a791c4c33ec79f90496d641f306a91fb112d3d060d5c1cee4e20 AS builder

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

FROM gcr.io/distroless/static:nonroot@sha256:0376b514983f02c630de9ed8abadd33968ddb778f9f383412a12babe639cbaaa
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
