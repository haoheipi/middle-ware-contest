FROM golang:latest
ADD middle-ware $GOPATH/src/middle-ware/
WORKDIR $GOPATH/src/middle-ware/client
RUN go build
WORKDIR $GOPATH/src/middle-ware/backened
RUN go build
COPY start.sh /start.sh
RUN chmod +x /start.sh
CMD ["/bin/bash","/start.sh"]
#ENTRYPOINT ["/bin/bash", "$GOPATH/src/start.sh"]
#ENTRYPOINT $GOPATH/src/start.sh && tail -f $GOPATH/src/start.sh
