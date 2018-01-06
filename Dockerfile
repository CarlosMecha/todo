FROM ubuntu:16.04

RUN apt-get update && apt-get install -y ca-certificates
ADD bin/server /bin/server
RUN chmod u+x /bin/server

ENTRYPOINT [ "server" ]