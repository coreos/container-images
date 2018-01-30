FROM scratch

MAINTAINER Lucas Serven <lucas.serven@coreos.com>

ARG ARCH

COPY bin/${ARCH}/tectonic-stats-extender /tectonic-stats-extender

ENTRYPOINT ["/tectonic-stats-extender"]
