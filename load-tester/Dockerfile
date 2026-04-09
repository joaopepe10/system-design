FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN go build -o load-tester .

FROM alpine:3.18
WORKDIR /app
COPY --from=build /app/load-tester .
EXPOSE 9090
ENTRYPOINT ["./load-tester"]
CMD ["-server"]
