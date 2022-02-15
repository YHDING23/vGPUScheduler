FROM debian:stretch-slim

WORKDIR /

COPY bin/vGPUScheduler /usr/local/bin

CMD ["vGPUScheduler"]