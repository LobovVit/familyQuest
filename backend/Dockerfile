FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/familyquest-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/familyquest-operator ./cmd/operator

FROM alpine:3.22

WORKDIR /app

RUN addgroup -S familyquest && adduser -S -G familyquest -h /app familyquest

COPY --from=build /bin/familyquest-api /usr/local/bin/familyquest-api
COPY --from=build /bin/familyquest-operator /usr/local/bin/familyquest-operator
COPY --chown=familyquest:familyquest migrations ./migrations

USER familyquest

EXPOSE 8081

CMD ["familyquest-api"]
