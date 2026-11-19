FROM golang:1.26 AS build
WORKDIR /src
COPY . .
RUN go build -o /hello-api .

FROM gcr.io/distroless/base-debian12
COPY --from=build /hello-api /hello-api
ENV PORT=8080
ENTRYPOINT ["/hello-api"]
