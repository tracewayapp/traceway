FROM golang:1.26 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /shop-api .

FROM gcr.io/distroless/static-debian12
COPY --from=build /shop-api /shop-api
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/shop-api"]
