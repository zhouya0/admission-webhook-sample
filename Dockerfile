FROM debian:stretch-slim

WORKDIR /

COPY _output/bin/admission-webhook /usr/local/bin

CMD ["admission-webhook"]