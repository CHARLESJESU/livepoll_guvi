FROM golang:1.22-alpine AS build
RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go mod tidy && go build -o /livepoll .
FROM alpine:3.20
COPY --from=build /livepoll /livepoll
EXPOSE 8080
CMD ["/livepoll"]
