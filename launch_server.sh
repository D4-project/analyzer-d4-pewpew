#!/bin/bash
go fmt
go build

rm pewpew.log
cp dump.rdb.fresh ./db/dump.rdb

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CONF="conf.sample"

screen -dmS "pewpew"
sleep 0.1

screen -S "pewpew" -X screen -t "pew-redis" bash -c "(${DIR}/redis/src/redis-server ${DIR}/redis.conf); read x;"
sleep 0.5
screen -S "pewpew" -X screen -t "pew-server" bash -c "(${DIR}/analyzer-d4-pewpew -c ${CONF}); read x;"
sleep 0.5
screen -S "pewpew" -X screen -t "pew-redis-cli" bash -c "(redis-cli -p 6501); read x;"
screen -S "pewpew" -X screen -t "pew-log" bash -c "(watch -n1 tail pewpew.log ); read x;"

exit 0
