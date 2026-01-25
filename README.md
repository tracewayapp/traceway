# Full documentation: https://docs.tracewayapp.com

# Technical documentation for running locally

## Project structure
There are 2 projects in this repo and one more project in the go-clients repo https://github.com/tracewayapp/go-client

backend
frontend

## Backend 

Represents the main backend for traceway. Stores stack traces, endpoint info and metrics into a clickhouse DB. Has an api that can be accessed from the frontend project. 

### How to run:

To run the backend you need to:
1. Install Go (1.25+)
2. Install ClickHouse
3. Install PostgreSQL
4. Create databases and users (instructions below)
5. Create the .env file (instructions below)
6. Open the backend folder and run `go run .`

### Database Setup

#### ClickHouse
1. Install ClickHouse
2. Create a database: `CREATE DATABASE traceway`
3. Use the default user or create one (default/empty password works for local dev)

#### PostgreSQL
1. Install PostgreSQL
2. Create a database and user:
```sql
CREATE USER traceway WITH PASSWORD 'your_password';
CREATE DATABASE traceway OWNER traceway;
```

### Backend .env file
Create a .env file in the backend folder with your database credentials:

```
JWT_SECRET=your-secret-key-minimum-32-characters-long
CLICKHOUSE_SERVER=localhost:9000
CLICKHOUSE_DATABASE=traceway
CLICKHOUSE_USERNAME=default
CLICKHOUSE_PASSWORD=
CLICKHOUSE_TLS=false
API_ONLY=false
PORTS=80,8082
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DATABASE=traceway
POSTGRES_USERNAME=traceway
POSTGRES_PASSWORD=your_password
POSTGRES_SSLMODE=disable
```

**JWT_SECRET**: Must be at least 32 characters. Used to sign JWT tokens for user authentication. Generate a secure random string for production.

### User Registration

When running locally for the first time:
1. Start the backend (`go run .` in the backend folder)
2. Start the frontend (`npm run dev` in the frontend folder)
3. Navigate to http://localhost:5173
4. The login page will automatically redirect you to the registration page since no users exist yet
5. Register your first user account
6. After registration, you'll be redirected to create your first organization and project

The client SDK connects using project-level tokens (not user credentials). After creating a project, you'll receive a token to use with the SDK.

## Frontend

Is a sveltekit app that is expected to run in the SPA mode (client running only). 

To run it:
1 - install node/npm
2 - run `npm install`
3 - run `npm run dev`

By default the frontend will connect to your backend on port 8082 if you're running your backend on a different port change the line `target: 'http://localhost:8082',` in the vite.config.ts

## go-client (https://github.com/tracewayapp/go-client)

This is a client that users would include in their app. Right now the only web framework supported is the gin framework.

To run the basic devtesting app you can cd into `testing/devtesting` and run `go run .` this will start the server located in devtesting.go the key code here is:
```
    endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	router := gin.Default()

	router.Use(tracewaygin.New(
		endpoint,
		tracewaygin.WithDebug(true),
		tracewaygin.WithRepanic(true),
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingUrl|tracewaygin.RecordingQuery|tracewaygin.RecordingHeader|tracewaygin.RecordingBody),
	))
```

Which sets up the code to upload to localhost:8082 with the token 'default_token_change_me' (this has to match your project). After that the code will start uploading your metrics/stracktraces to the backend, you should be able to see it in tableplus or any other clickhouse client you like using.

## Screenshot

<img width="2452" height="1966" alt="1" src="https://github.com/user-attachments/assets/30a4fa24-7d08-4b36-a8f3-42abc73692fd" />

