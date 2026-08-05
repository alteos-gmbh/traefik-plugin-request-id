# X-Request-ID plugin for Traefik

This plugin will add the X-Request-ID header with a generated UUIDv7 value to HTTP requests and responses, allowing downstream services to identify requests.

UUIDv7 (RFC 9562) embeds a Unix millisecond timestamp in the leading bits, so generated IDs sort chronologically as strings. That makes request IDs cluster well in log stores and time-ordered database indexes, while the remaining random bits keep them unguessable in practice.

Based upon:
- github.com/mdklapwijk/traefik-plugin-request-id
- github.com/pipe01/plugin-requestid
- github.com/gamblingpro/plugin-requestid
