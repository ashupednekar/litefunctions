# Runtime sources bundled into a tiny scratch image so CI pipelines can pull
# them from a registry instead of cloning the litefunctions repository.
#
# The image layout mirrors the repository subtree the workflows expect:
#   /build/runtimes   Dockerfiles and build inputs
#   /runtimes         runtime sources that functions are injected into
FROM scratch

COPY build/runtimes /build/runtimes
COPY runtimes /runtimes
