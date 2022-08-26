FROM ubuntu:jammy

ARG BINARY

#RUN apk add --no-cache ca-certificates
RUN mkdir /etc/karmada
COPY interpreter.wasm /etc/karmada
COPY libwasmer.so /usr/lib/libwasmer.so
COPY ${BINARY} /bin/${BINARY}
