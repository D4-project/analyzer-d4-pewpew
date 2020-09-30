#!/bin/bash
pushd pewpew
npm install
 pm run build
rsync -r --update build ../build
