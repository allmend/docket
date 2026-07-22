FROM mirror.gcr.io/library/golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Release workflow passes the tag (without the leading v); falls back to the
# literal in internal/version for plain local builds.
ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w ${VERSION:+-X github.com/allmend/docket/internal/version.Version=$VERSION}" \
      -o docket ./cmd/serve

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
