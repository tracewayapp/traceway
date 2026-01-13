# Go Client Gin Testing Container

Docker setup for testing the Traceway Go client SDK with the devtesting Gin application.

## Prerequisites

- Backend running on port 8082 (e.g., `go run .` in backend folder, or another container)
- Docker installed

## Build

```bash
docker build -t traceway-go-client-test ./testing/go-client-gin
```

## Run

```bash
docker run -d \
  --name go-client-test \
  -v $(pwd)/clients/go-client:/app \
  traceway-go-client-test
```

The Dockerfile sets `TRACEWAY_ENDPOINT` to use `host.docker.internal:8082` by default, which reaches services running on your Mac.

## Usage

Exec into the container:

```bash
docker exec -it go-client-test sh
```

Run the devtesting server:

```bash
go run .
```

The server will start and connect to the backend at `host.docker.internal:8082/api/report`.

## Custom Endpoint

Override the endpoint if needed:

```bash
docker run -d \
  --name go-client-test \
  -e TRACEWAY_ENDPOINT="your_token@http://your-host:8082/api/report" \
  -v $(pwd)/clients/go-client:/app \
  traceway-go-client-test
```

## Test Endpoints

Once running, test the SDK with these endpoints (from your Mac):

```bash
curl http://localhost:8080/test-ok
curl http://localhost:8080/test-not-found
curl http://localhost:8080/test-exception
curl http://localhost:8080/test-segments
```

## Cleanup

```bash
docker stop go-client-test && docker rm go-client-test
```
