#!/bin/bash

VERSION=$(cat VERSION)

echo "Current Version: $VERSION"

_build() {
	echo "Build and Package for $ARCH ..."

	rm -rf ./build
	strip ./rollbackup
	mkdir -p build/usr/local/bin
	cp rollbackup build/usr/local/bin/
	
	fpm -t deb -C build -s dir -n rollbackup -f -a $ARCH -v $VERSION .
	fpm -t rpm -C build -s dir -n rollbackup -f -a $ARCH -v $VERSION .

	if [ ! -z "$CIRCLE_ARTIFACTS" ]; then
		mv *.deb $CIRCLE_ARTIFACTS/
		mv *.rpm $CIRCLE_ARTIFACTS/
	fi
}

cd rollbackup
GOARCH=amd64 go build rollbackup.go
ARCH=amd64 _build

GOARCH=amd64 go build rollbackup.go
ARCH=i386 _build

