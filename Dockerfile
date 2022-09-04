FROM golang:1.18.2-alpine as build

WORKDIR /app/

COPY vendor .

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w" -a -o /app/server cmd/server/main.go

FROM alpine:latest

WORKDIR /app/
COPY --from=build /app/server .

EXPOSE 8000

ENTRYPOINT ["./server"]
