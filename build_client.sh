#!/bin/bash
pushd pewpew
npm install
npm run build
rsync -r --update build ../build
