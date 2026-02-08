# devtesting-nestjs

NestJS test application for the Traceway JS SDK (`@tracewayapp/nestjs`). Mirrors the Go `devtesting` reference app to validate that all SDK features work correctly in a NestJS environment.

## Running

```bash
npm install
npm run start:dev
```

The app starts on port **3001** by default (configurable via `PORT` env var).

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3001` | Server port |
| `TRACEWAY_ENDPOINT` | `default_token_change_me@http://localhost:8082/api/report` | Traceway connection string |

## Docker (PM2 cluster mode)

Runs 4 instances of the app via PM2 in cluster mode.

### Build

```bash
./docker-build.sh
```

This packs the local `@tracewayapp/*` packages into tarballs, builds the Docker image, then cleans up.

### Run

```bash
docker run -p 3001:3001 \
  -e TRACEWAY_ENDPOINT="ed37c05e51c54f1db721f6fc59994e9b@http://host.docker.internal:8082/api/report" \
  devtesting-nestjs
```

`host.docker.internal` resolves to the host machine's localhost, so the app inside Docker can reach the Traceway backend running on your local port 8082.

The NestJS app is exposed on `http://localhost:3001` as usual.

## SDK Integration

All `@tracewayapp/nestjs` integration points are used correctly:

| Component | Location | Usage |
|-----------|----------|-------|
| `TracewayModule.forRoot()` | `app.module.ts:14` | Configured with `connectionString`, `debug`, `onErrorRecording` |
| `TracewayMiddleware` | `app.module.ts:34` | Applied globally via `forRoutes("*")` |
| `TracewayExceptionFilter` | `app.module.ts:27` | Registered as global `APP_FILTER` |
| `TracewayService` | `app.controller.ts:21`, `app.service.ts:16` | Injected via constructor |
| `@Span()` decorator | `users.service.ts`, `app.service.ts` | Applied to 8 methods total |

### TracewayService Methods Used

| Method | Location | Purpose |
|--------|----------|---------|
| `captureMessage` | `app.service.ts:24` | Log informational messages |
| `captureException` | `app.service.ts:26` | Report errors |
| `captureExceptionWithAttributes` | `app.service.ts:71` | Report errors with custom attributes |
| `startSpan` / `endSpan` | `app.service.ts:45,51` | Manual span creation |
| `measureTask` | `app.service.ts:57` | Wrap background work as a tracked task |
| `setTraceAttribute` | `app.service.ts:79` | Attach context attributes to the current trace |

## Endpoints

### App Controller (14 endpoints)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/test-ok` | Returns `{ status: "ok" }` |
| GET | `/test-not-found` | Throws 404 HttpException |
| GET | `/test-exception` | Throws unhandled Error after random delay |
| GET | `/test-message` | Captures 10 messages + 1 exception via TracewayService |
| GET | `/test-spans` | Runs nested spans (db.query, cache.set, http.external_api) |
| GET | `/test-task` | Starts a background task via `measureTask` |
| GET | `/test-param/:param` | Returns the route param |
| GET | `/test-self-report-attributes` | Calls `captureExceptionWithAttributes` with custom attrs |
| GET | `/test-self-report-context` | Sets a trace attribute then captures an exception |
| GET | `/test-cerror-simple` | Throws a simple Error |
| GET | `/test-cerror-stacktrace` | Throws an Error from a nested function call |
| GET | `/test-cerror-wrapped` | Throws a 2-layer wrapped Error (using `cause`) |
| GET | `/test-cerror-custom` | Throws a CustomError with code + message |
| POST | `/test-recording/:param` | Accepts JSON body `{ name }`, throws if name != "good" |

### Users Controller (5 endpoints)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/users` | List all users |
| GET | `/users/:id` | Get user by ID |
| POST | `/users` | Create user (`{ firstName, lastName, email }`) |
| PUT | `/users/:id` | Update user |
| DELETE | `/users/:id` | Delete user |

All user service methods are decorated with `@Span()` (e.g. `db.users.findAll`, `db.users.create`) and use an in-memory array as the data store with simulated DB delay.

## Comparison with Go devtesting

### Shared Endpoints (15)

Both apps implement: `test-ok`, `test-not-found`, `test-exception`, `test-message`, `test-spans`, `test-task`, `test-param/:param`, `test-self-report-attributes`, `test-self-report-context`, `test-cerror-simple`, `test-cerror-stacktrace`, `test-cerror-wrapped`, `test-cerror-custom`, `test-recording/:param`, and `/users` CRUD.

### Go-Only Endpoints (7)

| Endpoint | Reason Not Ported |
|----------|-------------------|
| `/test-json` | Could be ported (not yet implemented) |
| `/test-50k` | Could be ported (not yet implemented) |
| `/test-self-report-attributes-panic` | Go-specific — uses `panic()` recovery |
| `/test-cerror-stacktrace-wrapped` | Go-specific — uses `NewStackTraceErrorf` API |
| `/test-cerror-multiple` | Go-specific — uses Gin `ctx.Error()` multi-error pattern |
| `/test-cerror-nested` | Could be ported (not yet implemented) |
| `/metrics` | Go-specific — uses `PrintCollectionFrameMetrics` API |

### Key Differences

| Aspect | Go devtesting | NestJS devtesting |
|--------|---------------|-------------------|
| DB layer | SQLite in-memory with `tracewaydb` wrapper | In-memory array with `@Span()` decorators |
| DB tracing | Automatic via `tracewaydb` | Manual via `@Span()` on service methods |
| Panic handling | `traceway.Recover()` + panic endpoints | NestJS exception filters (no panic concept) |
| Error wrapping | `traceway.NewStackTraceErrorf` | Native JS `Error.cause` chaining |
| Middleware | `traceway_gin.Use(router, ...)` | `TracewayMiddleware` applied via `NestModule.configure()` |
| Exception filter | Gin recovery middleware | `TracewayExceptionFilter` as global `APP_FILTER` |

## Project Structure

```
src/
├── main.ts                  # Bootstrap, listens on PORT
├── app.module.ts            # Root module — TracewayModule, middleware, exception filter
├── app.controller.ts        # 14 test endpoints
├── app.service.ts           # TracewayService usage (spans, tasks, errors)
└── users/
    ├── users.module.ts      # Users feature module
    ├── users.controller.ts  # CRUD endpoints
    ├── users.service.ts     # In-memory store with @Span() decorators
    └── user.entity.ts       # User, CreateUserDto, UpdateUserDto interfaces
```
