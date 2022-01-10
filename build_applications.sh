#!/bin/bash
GOOS=linux
GOARCH=amd64

if [ ! -z "$1" ]; then
  GOOS=$1
fi

if [ ! -z "$2" ]; then
  GOARCH=$2
fi

OUTDIR=bin/$GOOS/$GOARCH
export GO111MODULE=off
export GOOS=$GOOS
export GOARCH=$GOARCH
mkdir -p $OUTDIR
go build -o $OUTDIR ./cmd/*
