# thejoined

An HTTP diagnostic server (and closed-loop load-generator client) named `rna`, distributed as a single static binary and as container images.

## Language

### Publishing

**Publish**:
Building the container image and pushing it to a registry in one motion.
_Avoid_: deploy, release, ship

**Mirror**:
One of the peer repositories (cPacketNetworks and mlbright) holding the same code and history.
_Avoid_: fork, upstream/downstream

**Owner-derived publishing**:
The rule that each mirror publishes the image into its own GHCR namespace, and only its own.
_Avoid_: cross-publishing, dual-push

**Version tag**:
The image tag identifying the exact commit an image was built from, as reported by `git describe`.
_Avoid_: version number, semver, build number
