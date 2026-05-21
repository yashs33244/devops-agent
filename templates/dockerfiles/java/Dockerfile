# syntax=docker/dockerfile:1.10
# =============================================================================
# Java 21 — Multi-Stage Production Dockerfile
# Template variables (replace before use):
#   {{PORT}} — port the app listens on (default: 8080)
#
# NOTE: We use eclipse-temurin:21-jre-alpine as the final image (not distroless)
# because the JVM requires glibc/libz/libjvm.so and other shared libraries that
# are not present in a static distroless image. The alpine JRE is still minimal.
#
# To obtain real image digests run:
#   docker pull eclipse-temurin:21-jdk-alpine --no-trunc
#   docker pull eclipse-temurin:21-jre-alpine --no-trunc
# =============================================================================

# =============================================================================
# Stage 1 — builder
# Compiles the project with Maven OR Gradle (auto-detected by wrapper presence).
# Uses cache mounts for ~/.m2 and ~/.gradle to avoid re-downloading artifacts.
# =============================================================================
FROM eclipse-temurin:21-jdk-alpine@sha256:0000000000000000000000000000000000000000000000000000000000000007 AS builder
# ^ replace with actual digest from: docker pull eclipse-temurin:21-jdk-alpine --no-trunc

WORKDIR /app

# Copy build scripts / wrappers first so cache is busted only when they change.
COPY mvnw* gradlew* pom.xml* build.gradle* settings.gradle* ./
COPY .mvn* .gradle* ./

# Download dependencies into the build cache before copying source.
# Maven: use --mount=type=cache so ~/.m2 persists across builds.
# Gradle: same pattern with ~/.gradle.
RUN --mount=type=cache,target=/root/.m2 \
    --mount=type=cache,target=/root/.gradle \
    set -eux; \
    if [ -f mvnw ]; then \
      chmod +x mvnw && ./mvnw dependency:go-offline -B --no-transfer-progress; \
    elif [ -f gradlew ]; then \
      chmod +x gradlew && ./gradlew dependencies --no-daemon; \
    elif command -v mvn > /dev/null; then \
      mvn dependency:go-offline -B --no-transfer-progress; \
    fi

# Copy full source after deps to benefit from the cache layer above.
COPY src ./src

# Build the fat JAR, skipping tests (tests should run in CI, not during image build).
RUN --mount=type=cache,target=/root/.m2 \
    --mount=type=cache,target=/root/.gradle \
    set -eux; \
    if [ -f mvnw ]; then \
      ./mvnw package -DskipTests --no-transfer-progress -B; \
    elif [ -f gradlew ]; then \
      ./gradlew bootJar -x test --no-daemon; \
    elif command -v mvn > /dev/null; then \
      mvn package -DskipTests --no-transfer-progress -B; \
    else \
      gradle bootJar -x test --no-daemon; \
    fi

# Normalise the JAR output path so the runner stage can COPY from a fixed location.
RUN set -eux; \
    mkdir -p /dist; \
    find /app/target /app/build/libs -maxdepth 1 -name "*.jar" \
      ! -name "*sources*" ! -name "*javadoc*" \
      -exec cp {} /dist/app.jar \; 2>/dev/null || true; \
    [ -f /dist/app.jar ] || (echo "No JAR found in target/ or build/libs/" && exit 1)

# =============================================================================
# Stage 2 — runner (final image)
# eclipse-temurin:21-jre-alpine is a minimal JRE — no JDK tools, no compiler.
# A dedicated non-root user (appuser/appgroup) is created for security.
# =============================================================================
FROM eclipse-temurin:21-jre-alpine@sha256:0000000000000000000000000000000000000000000000000000000000000008 AS runner
# ^ replace with actual digest from: docker pull eclipse-temurin:21-jre-alpine --no-trunc

WORKDIR /app

# Create a dedicated non-root group and user. -S = system account (no password/home).
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy only the executable JAR — no source, no build artefacts.
COPY --from=builder --chown=appuser:appgroup /dist/app.jar ./app.jar

# Drop privileges before starting the JVM.
USER appuser

EXPOSE {{PORT}}

# -XX:+UseContainerSupport — JVM respects cgroup CPU/memory limits (default in JDK 11+,
#   explicit here for clarity).
# -XX:MaxRAMPercentage=75.0 — JVM heap can use up to 75% of container memory limit.
# -XX:+ExitOnOutOfMemoryError — crash fast; let Kubernetes restart rather than limp along.
HEALTHCHECK --interval=30s --timeout=10s --start-period=45s --retries=3 \
  CMD wget -qO- http://localhost:{{PORT}}/actuator/health 2>/dev/null | grep -q '"status":"UP"' || exit 1

ENTRYPOINT ["java", \
  "-XX:+UseContainerSupport", \
  "-XX:MaxRAMPercentage=75.0", \
  "-XX:+ExitOnOutOfMemoryError", \
  "-Djava.security.egd=file:/dev/./urandom", \
  "-jar", "app.jar"]
