FROM golang:1.23.2-alpine AS build
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /gofins
FROM golang:1.23.2-alpine
WORKDIR /
COPY --from=build /gofins /gofins
ENTRYPOINT ["/gofins"]
LABEL Name=gofins Version=0.0.1