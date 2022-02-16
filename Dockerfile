FROM debian:stretch-slim

WORKDIR /

COPY vGPUScheduler /usr/local/bin

CMD ["vGPUScheduler"]