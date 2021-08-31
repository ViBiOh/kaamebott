FROM vibioh/scratch

ENV API_PORT 1080
EXPOSE 1080

ENV ZONEINFO /zoneinfo.zip
COPY zoneinfo.zip /zoneinfo.zip
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

HEALTHCHECK --retries=10 CMD [ "/api", "-url", "http://localhost:1080/health" ]
ENTRYPOINT [ "/api" ]

ARG VERSION
ENV VERSION=${VERSION}

ARG TARGETOS
ARG TARGETARCH

COPY release/api_${TARGETOS}_${TARGETARCH} /api
COPY release/discord_${TARGETOS}_${TARGETARCH} /discord
