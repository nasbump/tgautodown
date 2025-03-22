FROM ubuntu:18.04
RUN apt-get update && apt-get install -y ca-certificates 
COPY ./bin /app/
WORKDIR /app
ENTRYPOINT ["/app/tgautodown", "-dir", "/download", "-gopeed", "/app/gopeed"]
