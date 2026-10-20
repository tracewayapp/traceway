FROM eclipse-temurin:21-jre
WORKDIR /app
ADD https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.20.1/opentelemetry-javaagent.jar /app/opentelemetry-javaagent.jar
COPY target/admin-api-3.18.1.jar /app/app.jar
ENV OTEL_SERVICE_NAME=admin-api \
    OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
    OTEL_INSTRUMENTATION_COMMON_EXPERIMENTAL_CONTROLLER_TELEMETRY_ENABLED=true \
    OTEL_INSTRUMENTATION_LOGBACK_APPENDER_EXPERIMENTAL_CAPTURE_KEY_VALUE_PAIR_ATTRIBUTES=true \
    JAVA_TOOL_OPTIONS="-javaagent:/app/opentelemetry-javaagent.jar"
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
