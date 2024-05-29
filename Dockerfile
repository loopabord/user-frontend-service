# Stage 1: Build the Go application
FROM golang:1.22.3-alpine AS build

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files to the working directory
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the rest of the application code to the working directory
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Create a lightweight container with the Go application
FROM alpine:latest

# Install ca-certificates for HTTPS support
RUN apk --no-cache add ca-certificates

# Set the working directory inside the container
WORKDIR /root/

# Copy the built Go application from the previous stage
COPY --from=build /app/main .

# Expose the port the service will run on
EXPOSE 8080

# Command to run the Go application
CMD ["./main"]