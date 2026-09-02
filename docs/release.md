# Releasing audible-mcp

## Tag format

Semver with a `v` prefix: `v2.0.0`, `v2.1.0`, `v2.0.1`.

The first public release of the Go rewrite is `v2.0.0`.

## What a tag publishes

Pushing a `v*` tag to `main`'s remote runs `.github/workflows/release.yml`. GoReleaser then:

1. Cross-compiles `audible-mcp` for linux/darwin/windows × amd64/arm64
2. Uploads archives and `checksums.txt` to a GitHub Release
3. Builds and pushes `ghcr.io/mfacenet/audible-mcp:<version>` (amd64 + arm64 manifest) and `:latest`

The image is non-root and labeled `io.modelcontextprotocol.server.name=io.github.mfacenet/audible-mcp`. It does not contain an auth file.

## Checklist before tagging

1. `main` is green: unit tests, vet, and the GoReleaser **snapshot** job (`goreleaser build --snapshot --clean`)
2. README install commands match the version you are about to cut
3. You are ready to set the GHCR package public if this is the first image push (GitHub Packages → `audible-mcp` → Package settings → Change visibility)

## Cut a release

```sh
git checkout main
git pull
git tag -a v2.0.0 -m "v2.0.0"
git push origin v2.0.0
```

Do not have the PR workflow create tags.

## Yank a bad release

1. Do not push further tags until the problem is understood
2. GitHub Releases → delete the release (the git tag can stay or be deleted)
3. GHCR → delete the version tag if the image is wrong
4. Ship a `v2.0.1` (or whatever the next patch is) from a fix on `main`

Deleting a tag does not remove a Go module version that clients have already fetched. Prefer a new patch tag over rewriting history.
