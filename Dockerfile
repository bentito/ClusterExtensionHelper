# Use an official Golang image as the build environment
FROM golang:1.22 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to the workspace
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the webhook binary
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook cmd/main.go

# Use a minimal base image for the final image
FROM alpine:3.14

# Set the working directory inside the container
WORKDIR /app

# Copy the webhook binary from the builder stage
COPY --from=builder /app/webhook .

# Expose the port that the webhook server listens on
EXPOSE 8443

# Command to run when the container starts
CMD ["./webhook"]
