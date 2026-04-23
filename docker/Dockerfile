FROM mirror.gcr.io/library/golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o docket ./cmd/serve

# ---

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/docket      /docket
COPY --from=builder /app/migrations  /migrations
COPY --from=builder /app/templates   /templates
COPY --from=builder /app/static/dist /static/dist

EXPOSE 8081
USER nonroot:nonroot

ENTRYPOINT ["/docket"]
CMD ["--mode=all"]
