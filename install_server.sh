#!/bin/bash

set -e
set -x

# Go #
sudo apt-get install screen -y
sudo add-apt-repository ppa:longsleep/golang-backports -y
sudo apt update
sudo apt install golang-go -y
go get -u
go build

# REDIS #
mkdir -p db
test ! -d redis/ && git clone https://github.com/antirez/redis.git
pushd redis/
git checkout 5.0
make
popd
