# syntax=docker/dockerfile:1.7
# GoReleaser copies the pre-built linux binary into the build context as audible-mcp.
# Alpine is used only to obtain CA certificates for TLS to Amazon/Audible.

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
ARG TARGETPLATFORM
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY ${TARGETPLATFORM}/audible-mcp /audible-mcp
USER 65532:65532
LABEL org.opencontainers.image.source="https://github.com/mfacenet/audible-mcp" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="audible-mcp" \
      org.opencontainers.image.description="MCP server for authenticated Audible library reads" \
      io.modelcontextprotocol.server.name="io.github.mfacenet/audible-mcp"
ENTRYPOINT ["/audible-mcp"]
CMD ["serve"]
