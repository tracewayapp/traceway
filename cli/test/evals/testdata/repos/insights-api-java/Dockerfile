FROM eclipse-temurin:21-jre
WORKDIR /app
ADD https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.20.1/opentelemetry-javaagent.jar /app/opentelemetry-javaagent.jar
COPY build/libs/insights-api-2.31.0.jar /app/app.jar
ENV OTEL_SERVICE_NAME=insights-api \
    OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
    OTEL_METRIC_EXPORT_INTERVAL=60000 \
    OTEL_INSTRUMENTATION_LOGBACK_APPENDER_EXPERIMENTAL_CAPTURE_KEY_VALUE_PAIR_ATTRIBUTES=true \
    JAVA_TOOL_OPTIONS="-javaagent:/app/opentelemetry-javaagent.jar -Xms4g -Xmx4g -XX:+UseG1GC"
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
