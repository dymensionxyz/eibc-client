FROM golang:1.23

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the Go app
RUN go build -o eibc-client .
RUN cp /app/eibc-client /usr/local/bin/eibc-client