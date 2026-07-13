# Owner-derived GHCR publishing

The image must be published to both `ghcr.io/cpacketnetworks/thejoined` and `ghcr.io/mlbright/thejoined`. Rather than having one workflow run push to both namespaces, the workflow derives the namespace from the repository owner it runs in (`github.repository_owner`, lowercased): each mirror publishes its own image using only the built-in `GITHUB_TOKEN`. We chose this because it needs zero managed secrets, is fork-portable, and cannot rot — the alternative would put one mirror's credentials inside the other.

## Considered Options

- **Owner-derived (chosen)** — same workflow file publishes `cpacketnetworks/...` when run in the cPacketNetworks mirror and `mlbright/...` when run in the mlbright mirror.
- **One run pushes both** — rejected: requires a classic PAT with `write:packages` stored as a repo secret, which can expire and silently break publishing, and gives whichever repo hosts it write access to the other namespace.

## Consequences

The two images stay in lockstep only as long as commits are pushed to both mirrors. A mirror that stops receiving pushes serves a stale image; nothing in CI detects or corrects that drift.
