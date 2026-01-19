# Temporary documentation

## Project structure
There are 2 projects in this repo and one more project in the go-clients repo https://github.com/tracewayapp/go-client

backend
frontend

## Backend 

Represents the main backend for traceway. Stores stack traces, endpoint info and metrics into a clickhouse DB. Has an api that can be accessed from the frontend project. 

### How to run:

To run the backend you need to:
1 - install golang
2 - install clickhosue
3 - create a clickhouse database (eg: traceway)
4 - create a clickhouse user (eg: if one does not exist default/empty password)
5 - create the .env file (instructions below)
6 - open the backend folder and run `go run .`

### Backend .env file
To run it you need to create a .env file in the backend project locally with your own clickhouse credentials.

Example .env:

```
APP_TOKEN="nice"
CLICKHOUSE_SERVER=localhost:9000
CLICKHOUSE_DATABASE=traceway
CLICKHOUSE_USERNAME=default
CLICKHOUSE_PASSWORD=
CLICKHOUSE_TLS=false
API_ONLY=false
PORTS=80,8082
```

The APP_TOKEN value is what you will use to login when running the frontend. The client app will connect using the project level token, the default project is created in the migration 0009_insert_default_project.up.sql with the TOKEN value of default_token_change_me.

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

