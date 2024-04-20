FROM golang:1.22

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the Go app
RUN go build -o bot .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./bot"]
