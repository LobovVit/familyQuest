FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/familyquest-api ./cmd/api

FROM alpine:3.22

WORKDIR /app

COPY --from=build /bin/familyquest-api /usr/local/bin/familyquest-api
COPY migrations ./migrations

EXPOSE 8081

CMD ["familyquest-api"]
