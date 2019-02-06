#!/bin/bash
VERSION_FILE=VERSION

if ! test -f $VERSION_FILE; then echo "0-0" > $VERSION_FILE; fi

if [[ -z "${CI}" ]];
then
    echo `git rev-parse --short HEAD`-$(($(cat $VERSION_FILE|awk -F "-" '{print $2}') + 1)) > $VERSION_FILE
else
    echo "CI"
fi
