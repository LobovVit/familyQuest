FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
ARG APP_BUILT_AT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X github.com/lobov/familyquest/backend/internal/buildinfo.Version=${APP_VERSION} -X github.com/lobov/familyquest/backend/internal/buildinfo.Commit=${APP_COMMIT} -X github.com/lobov/familyquest/backend/internal/buildinfo.BuiltAt=${APP_BUILT_AT}" -o /bin/familyquest-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/familyquest-operator ./cmd/operator

FROM alpine:3.22

ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
ARG APP_BUILT_AT=unknown
LABEL org.opencontainers.image.version=$APP_VERSION org.opencontainers.image.revision=$APP_COMMIT org.opencontainers.image.created=$APP_BUILT_AT

WORKDIR /app

RUN addgroup -S familyquest && adduser -S -G familyquest -h /app familyquest

COPY --from=build /bin/familyquest-api /usr/local/bin/familyquest-api
COPY --from=build /bin/familyquest-operator /usr/local/bin/familyquest-operator
COPY --chown=familyquest:familyquest migrations ./migrations

USER familyquest

EXPOSE 8081

CMD ["familyquest-api"]
