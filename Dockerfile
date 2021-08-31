FROM vibioh/scratch

ENV API_PORT 1080
EXPOSE 1080

ENV ZONEINFO /zoneinfo.zip
COPY zoneinfo.zip /zoneinfo.zip
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

HEALTHCHECK --retries=10 CMD [ "/kaamebott", "-url", "http://localhost:1080/health" ]
ENTRYPOINT [ "/kaamebott" ]

ARG VERSION
ENV VERSION=${VERSION}

ARG TARGETOS
ARG TARGETARCH

COPY release/kaamebott_${TARGETOS}_${TARGETARCH} /kaamebott
COPY release/discord_${TARGETOS}_${TARGETARCH} /discord
