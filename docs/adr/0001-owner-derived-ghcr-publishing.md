# Owner-derived GHCR publishing

The image must be published to both `ghcr.io/cpacketnetworks/thejoined` and `ghcr.io/mlbright/thejoined`. Rather than having one workflow run push to both namespaces, the workflow derives the namespace from the repository owner it runs in (`github.repository_owner`, lowercased): each mirror publishes its own image using only the built-in `GITHUB_TOKEN`. We chose this because it needs zero managed secrets, is fork-portable, and cannot rot — the alternative would put one mirror's credentials inside the other.

## Considered Options

- **Owner-derived (chosen)** — same workflow file publishes `cpacketnetworks/...` when run in the cPacketNetworks mirror and `mlbright/...` when run in the mlbright mirror.
- **One run pushes both** — rejected: requires a classic PAT with `write:packages` stored as a repo secret, which can expire and silently break publishing, and gives whichever repo hosts it write access to the other namespace.

## Consequences

The two images stay in lockstep only as long as commits are pushed to both mirrors. A mirror that stops receiving pushes serves a stale image; nothing in CI detects or corrects that drift.

`GITHUB_TOKEN` can only write a package that GHCR has linked to the repository the workflow runs in. GHCR makes that link automatically when the workflow itself creates the package (the `org.opencontainers.image.source` label names the repo), but never retroactively. A package that already exists in the namespace, for example one first pushed by hand with a personal token, rejects the workflow with `denied: permission_denied: write_package` until the package owner does one of the following once, in the package's settings page (`https://github.com/users/<owner>/packages/container/thejoined/settings` for a user, `https://github.com/orgs/<owner>/packages/container/thejoined/settings` for an org):

- **Manage Actions access → Add repository** `<owner>/thejoined` with the **Write** role (keeps existing tags), or
- **Delete this package**, so the next workflow run recreates it already linked.

The mlbright mirror hit exactly this: `ghcr.io/mlbright/thejoined` was created by local `make publish` runs in 2026-03/04, so every CI publish there failed until access was granted.
