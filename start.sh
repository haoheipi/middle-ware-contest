if     [ $SERVER_PORT -le 8001 ] ;
then
   $GOPATH/src/middle-ware/client/client $SERVER_PORT &
else
   $GOPATH/src/middle-ware/backened/backened $SERVER_PORT &
fi
#/bin/bash
tail -f /start.sh
