FROM scratch
ARG EXT=
COPY statesaver${EXT} /statesaver
ENTRYPOINT ["/statesaver"]
