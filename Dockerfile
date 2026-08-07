# Holds this plugin's source, including the vendored dependencies Traefik's
# local-plugin loader requires, at the path it expects: plugins-local/src/
# <module path>. Meant to run as a Kubernetes init container that copies
# that tree onto a volume shared with the Traefik container — see the
# "Kubernetes init container" section in README.md for the full example.
# There is nothing to execute at runtime, so no ENTRYPOINT/CMD.
FROM alpine:3.24

COPY . /plugins-local/src/github.com/alteos-gmbh/traefik-plugin-request-id/
