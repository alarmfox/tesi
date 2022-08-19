FROM golang:1.18.2-alpine as build

WORKDIR /app/

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w" -a -o /app/main cmd/server/main.go

FROM alpine:latest

WORKDIR /app/
COPY --from=build /app/main .

EXPOSE 8000

ENTRYPOINT ["./main"]
