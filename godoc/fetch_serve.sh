#!/bin/bash

HOST=$1
ORG=$2
REPO=$3
PR=$4

[ "$HOST" == "" ] && ( echo "no host specified"; exit 1; )
[ "$ORG" == "" ] && ( echo "no org specified"; exit 1; )
[ "$REPO" == "" ] && ( echo "no repo specified"; exit 1; )
[ "$PR" == "" ] && ( echo "no pr specified"; exit 1; )

mkdir -p src/$HOST/$ORG \
 && cd src/$HOST/$ORG/ \
 && git clone --dept=1 https://$HOST/$ORG/$REPO \
 && cd $REPO \
 && git fetch origin pull/$PR/head:local_branch \
 && git checkout local_branch \
 && godoc -goroot /usr/local/go -http=:6060
