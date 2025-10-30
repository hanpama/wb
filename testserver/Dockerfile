FROM golang:1.25-bookworm

WORKDIR /app

# Copy go mod file
COPY go.mod ./

# Copy source code
COPY *.go ./

# Copy templates and testflows
COPY templates/ ./templates/
COPY testflows/ ./testflows/

# Build the application
RUN go build -o testserver .

# Expose port 8080
EXPOSE 8080

# Run the testserver
CMD ["./testserver"]
