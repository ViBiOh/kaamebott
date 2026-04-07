FROM rg.fr-par.scw.cloud/vibioh/scratch

ENV API_PORT=1080
EXPOSE 1080

COPY cacert.pem /etc/ssl/cert.pem

HEALTHCHECK --retries=10 CMD [ "/kaamebott", "-url", "http://127.0.0.1:1080/health" ]
ENTRYPOINT [ "/kaamebott" ]

ARG VERSION
ENV VERSION=${VERSION}

ARG GIT_SHA
ENV GIT_SHA=${GIT_SHA}

ARG TARGETOS
ARG TARGETARCH

COPY release/kaamebott_${TARGETOS}_${TARGETARCH} /kaamebott
