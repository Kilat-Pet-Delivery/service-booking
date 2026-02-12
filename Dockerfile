FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build
COPY lib-common ./lib-common
COPY lib-proto ./lib-proto
COPY service-booking ./service-booking
WORKDIR /build/lib-common
RUN go mod download
WORKDIR /build/lib-proto
RUN go mod download
WORKDIR /build/service-booking
RUN go mod download && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Kuala_Lumpur
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/server .
COPY service-booking/migrations ./migrations
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8001
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8001/health || exit 1
CMD ["./server"]
