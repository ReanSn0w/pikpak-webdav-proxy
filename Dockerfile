FROM golang:1.25-alpine AS application
ARG REF=bundle
ADD . /bundle
WORKDIR /bundle
RUN \
    echo "Prepearing Environment." && \
    apk --no-cache add ca-certificates
RUN \
    version=${REF} && \
    echo "Building service. Version: ${version}" && \
    go build -ldflags "-X main.revision=${version}" -o /srv/app ./main.go

# Финальная сборка образа
FROM scratch
COPY --from=application /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=application /srv /srv
ENV PORT=8080
EXPOSE 8080
WORKDIR /srv
ENTRYPOINT ["/srv/app"]
