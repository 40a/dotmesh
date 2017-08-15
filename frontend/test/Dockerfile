FROM alpine:3.4

MAINTAINER Sebastian Tschan <mail@blueimp.net>

RUN apk --no-cache add \
    # Install tini, a tiny but valid init for containers:
    tini \
    # Install NodeJS:
    nodejs-lts \
    # Install ffmpeg for video recording:
    ffmpeg \
  && npm install -g \
    # Install Nightwatch.js:
    nightwatch@'<1.0' \
  # Clean up obsolete files:
  && rm -rf \
    /tmp/* \
    /root/.npm

ADD ./wait-for.sh /usr/local/bin/wait-for
RUN chmod a+x /usr/local/bin/wait-for

ADD ./entrypoint.sh /usr/local/bin/entrypoint
RUN chmod a+x /usr/local/bin/entrypoint

ADD . /home/node

WORKDIR /home/node

ENTRYPOINT ["entrypoint"]