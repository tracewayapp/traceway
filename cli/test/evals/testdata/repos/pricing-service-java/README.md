# pricing-service

Prices carts for checkout (Spring Boot 3, Java 21). `POST /api/prices/quote`
takes cart lines and returns unit and total prices after promotions;
`GET /api/prices/{sku}` returns one list price.

Runs as three replicas on Kubernetes (`k8s/deployment.yaml`); releases are a
rolling update. Traces and logs go to Traceway through the OpenTelemetry
Java agent (`-javaagent:/otel/opentelemetry-javaagent.jar`).

```
./mvnw test
./mvnw spring-boot:run
```
