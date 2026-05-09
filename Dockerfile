FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/eaton-m3-exporter ./cmd/eaton-m3-exporter

FROM gcr.io/distroless/base-debian13:nonroot

WORKDIR /app

COPY --from=builder /out/eaton-m3-exporter /app/eaton-m3-exporter

EXPOSE 9734

USER nonroot:nonroot

ENTRYPOINT ["/app/eaton-m3-exporter"]
CMD ["--config", "/app/config.yaml", "--log-level", "info"]
