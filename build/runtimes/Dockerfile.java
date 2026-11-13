FROM maven:3.9-eclipse-temurin-17 AS builder

WORKDIR /app

COPY pom.xml ./
RUN mvn -q -B -Dmaven.test.skip=true dependency:go-offline

COPY src ./src
RUN mvn -q -B -Dmaven.test.skip=true package

FROM eclipse-temurin:17-jre

WORKDIR /app

ENV HTTP_PORT=8080
ENV USE_TELEMETRY=false

COPY --from=builder /app/target/*.jar /app/app.jar

EXPOSE 8080

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
