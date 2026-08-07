# X-Request-ID plugin for Traefik

This plugin will add the X-Request-ID header with a generated UUIDv7 value to HTTP requests and responses, allowing downstream services to identify requests.

UUIDv7 (RFC 9562) embeds a Unix millisecond timestamp in the leading bits, so generated IDs sort chronologically as strings. That makes request IDs cluster well in log stores and time-ordered database indexes, while the remaining random bits keep them unguessable in practice.

Strict ordering holds within a single Traefik process, where a sequence counter breaks ties inside the same millisecond. Across processes or hosts, ordering is approximate: it is only as good as the clock skew between them, and IDs generated in the same millisecond have no defined order. Treat the ordering as a locality property useful for storage and browsing, not as a guarantee to build logic on.

If an ID must not be recoverable in time, note that UUIDv7 deliberately exposes its creation timestamp. It also carries less entropy than UUIDv4: 62 random bits against UUIDv4's 122. RFC 9562 permits the 12 bits after the version nibble to be random, but `google/uuid` uses them for the process-local sequence counter that breaks ties within a millisecond, so they are clock-derived rather than random.

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `headerName` | string | `X-Request-ID` | Header to read and set. |
| `enabled` | bool | `true` | When false, the plugin passes requests through untouched. |

An incoming request that already carries the header is left alone, so IDs assigned upstream survive.

## Installation

The plugin is distributed as a local plugin: Traefik reads the sources from disk and interprets them, so there is no registry to install from. Deploy it by making a tagged version available to Traefik at `/plugins-local/src/github.com/alteos-gmbh/traefik-plugin-request-id`.

### 1. Fetch a release

```bash
VERSION=v1.0.0
DEST=plugins-local/src/github.com/alteos-gmbh/traefik-plugin-request-id

# Replace rather than merge, so an upgrade cannot leave stale files behind.
rm -rf "$DEST"
mkdir -p "$DEST"
curl -fsSL "https://github.com/alteos-gmbh/traefik-plugin-request-id/archive/refs/tags/${VERSION}.tar.gz" \
  | tar -xz --strip-components=1 -C "$DEST"
```

The directory name has to match the module path exactly. Traefik resolves the plugin by that path and reports a load error if it differs.

`--strip-components=1` drops the `traefik-plugin-request-id-<version>/` prefix GitHub puts in its archives, so the sources land directly in `$DEST`.

In an image build, copy the sources in instead:

```dockerfile
FROM traefik:v3.5
COPY plugins-local/ /plugins-local/
```

#### Kubernetes init container

Kubernetes 1.36 can [mount an OCI image straight into a pod as a volume](https://kubernetes.io/docs/concepts/storage/volumes/#image), which would make this unnecessary, but on earlier versions the plugin has to be copied onto a shared volume by an init container instead.

Every tagged release also publishes `ghcr.io/alteos-gmbh/traefik-plugin-request-id:<version>`: an image that carries nothing but this plugin's sources (including its vendored dependencies) at `/plugins-local/src/github.com/alteos-gmbh/traefik-plugin-request-id`, built from the `Dockerfile` in this repo. An init container copies that directory onto an `emptyDir` shared with the Traefik container, which mounts it at the path Traefik's local-plugin loader expects:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: traefik
spec:
  initContainers:
    - name: copy-requestid-plugin
      image: ghcr.io/alteos-gmbh/traefik-plugin-request-id:v1.0.0
      command: ["sh", "-c", "cp -r /plugins-local/. /out/"]
      volumeMounts:
        - name: plugins-local
          mountPath: /out

  containers:
    - name: traefik
      image: traefik:v3.5
      args:
        - --experimental.localPlugins.requestid.moduleName=github.com/alteos-gmbh/traefik-plugin-request-id
      volumeMounts:
        - name: plugins-local
          mountPath: /plugins-local

  volumes:
    - name: plugins-local
      emptyDir: {}
```

The `cp` source is `/plugins-local/.`, not `/plugins-local/src/.`: the image's `src/` directory has to survive the copy, because Traefik looks for the module under `<plugins-local mount>/src/<module path>`. Copying only `src/`'s contents would drop that level and leave Traefik unable to find the plugin.

### 2. Register it in the static configuration

```yaml
experimental:
  localPlugins:
    requestid:
      moduleName: github.com/alteos-gmbh/traefik-plugin-request-id
```

Or via CLI flags:

```text
--experimental.localPlugins.requestid.moduleName=github.com/alteos-gmbh/traefik-plugin-request-id
```

### 3. Declare the middleware in the dynamic configuration

```yaml
http:
  middlewares:
    request-id:
      plugin:
        requestid:
          headerName: X-Request-ID
          enabled: true

  routers:
    my-router:
      rule: Host(`example.com`)
      service: my-service
      middlewares:
        - request-id

  services:
    my-service:
      loadBalancer:
        servers:
          - url: http://backend:80
```

The key under `plugin` (`requestid` above) must match the name used in `localPlugins`.

### Docker Compose example

This uses the file provider so the example stays self-contained, with `dynamic.yml` holding the middleware, router and service definitions from step 3. The `backend` container is what `my-service` points at; `whoami` echoes the request headers back, which makes the injected ID easy to see.

```yaml
services:
  traefik:
    image: traefik:v3.5
    command:
      - --providers.file.filename=/etc/traefik/dynamic.yml
      - --entrypoints.web.address=:80
      - --experimental.localPlugins.requestid.moduleName=github.com/alteos-gmbh/traefik-plugin-request-id
    ports:
      - "80:80"
    volumes:
      - ./dynamic.yml:/etc/traefik/dynamic.yml:ro
      - ./plugins-local/src/github.com/alteos-gmbh/traefik-plugin-request-id:/plugins-local/src/github.com/alteos-gmbh/traefik-plugin-request-id:ro

  backend:
    image: traefik/whoami
```

The service name in `dynamic.yml` and the container it targets have to line up: `my-service` resolves to `http://backend:80`, which is the `backend` service above.

```bash
curl -si -H 'Host: example.com' http://localhost/ | grep -i x-request-id
```

That prints the header twice: once from the response, and once from the request as `whoami` received it.

If you use the Docker provider instead, be aware that mounting `/var/run/docker.sock` into Traefik grants it the Docker API, which is equivalent to control over the host. Mounting the socket `:ro` does not limit this, because the API is reached through the socket rather than by writing to the file. For production, put a socket proxy with an operation allowlist in front of it, or use a provider that does not need the socket at all.

## Releasing

Releases happen on merge to `master`. Nothing needs tagging by hand: the release workflow runs the test suite under both the Go toolchain and Yaegi, works out the next version, and creates the tag and GitHub release itself. Consumers pin to that tag in step 1 above.

The version comes from the most recent `v*` tag, bumped according to the [Conventional Commits](https://www.conventionalcommits.org) in the merged commits:

| Commits since the last tag | Bump | Example |
| --- | --- | --- |
| A `!` after the type, or a `BREAKING CHANGE:` body | major | `feat!: rename headerName` → `v2.0.0` |
| A `feat:` commit | minor | `feat: add config option` → `v1.1.0` |
| Anything else | patch | `fix: handle nil header` → `v1.0.1` |

Merges that only touch documentation, CI, tests or the licence do not release, so nobody gets a new version for a typo fix. Add `[skip release]` to the merge commit to suppress a release explicitly, or run the workflow manually with a `version` input to publish an exact version.

The logic lives in `.github/scripts/next-version.sh` rather than inline YAML so it can be run and tested directly:

```bash
.github/scripts/next-version.sh
# version=v1.0.1
# release=true
# reason=patch bump from v1.0.0
```

Traefik requires dependencies to be vendored, so run `make vendor` and commit `vendor/` whenever `go.mod` changes. CI fails if the two drift apart.

### Why not the public Traefik Plugin Catalog

The catalog does not accept forks, and this repository is a fork of `mdklapwijk/traefik-plugin-request-id`. Listing it on [plugins.traefik.io](https://plugins.traefik.io) would require detaching the fork relationship through GitHub Support, or re-creating the repository as a standalone one, and then adding the `traefik-plugin` topic. The local plugin route above needs none of that and keeps the plugin private to the organisation.

## Development

Requires Go 1.26 or newer, which is what `go.mod` declares and what CI runs.

```bash
make test        # gofmt, go vet, go test
make yaegi-test  # run the suite under the Yaegi interpreter Traefik uses
make vendor      # refresh vendor/ after changing go.mod
```

`make yaegi-test` matters because Traefik interprets plugins rather than compiling them. Code that builds cleanly can still fail to load, so treat the Yaegi run as the authoritative check. It requires [Yaegi](https://github.com/traefik/yaegi) on `PATH`:

```bash
go install github.com/traefik/yaegi/cmd/yaegi@v0.16.1
```

Based upon:

- github.com/mdklapwijk/traefik-plugin-request-id
- github.com/pipe01/plugin-requestid
- github.com/gamblingpro/plugin-requestid
