#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CONF="conf.sample"

screen -dmS "pewpew"
sleep 0.1

screen -S "pewpew" -X screen -t "pew-redis" bash -c "(${DIR}/redis/src/redis-server ${DIR}/redis.conf); read x;"
screen -S "pewpew" -X screen -t "pew-server" bash -c "(${DIR}/analyzer-d4-pewpew -c ${CONF}); read x;"

exit 0
