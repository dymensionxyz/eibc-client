FROM golang:1.22

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the Go app
RUN go build -o eibc-client .

# Command to run the executable
CMD ["./eibc-client", "start"]
