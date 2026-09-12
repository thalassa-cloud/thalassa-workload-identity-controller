FROM golang:1.27-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/manager ./cmd

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/manager /thalassa-workload-identity-controller
USER nonroot:nonroot
EXPOSE 8080 8081
ENTRYPOINT ["/thalassa-workload-identity-controller"]
