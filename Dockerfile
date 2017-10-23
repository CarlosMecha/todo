FROM ubuntu:16.04

MAINTAINER CarlosMecha

RUN apt-get update && apt-get install -y ca-certificates
ADD server.crt server.key /etc/
EXPOSE 6060

ADD bin/server /bin/server
RUN chmod u+x /bin/server

ENTRYPOINT [ "server", "-cert=/etc/server.crt", "-cert-key=/etc/server.key" ]