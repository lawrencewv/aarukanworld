# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/aarukanworld .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=build /out/aarukanworld /app/aarukanworld

ENV PORT=8080
ENV DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]

USER nobody
CMD ["/app/aarukanworld"]
