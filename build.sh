#!/bin/bash

VERSION=$(cat VERSION)

echo "Current Version: $VERSION"

fpm_flags="--verbose --license GPLv3 --vendor RollBackup.com --url https://rollbackup.com/ --maintainer dist@rollbackup.com --before-install ../before-install.sh --depends rsync"
BUILD_DIR=/tmp/rollbackup-agent-build

_build() {
	echo "Build and Package for $ARCH ($BUILD_DIR) ..."

	rm -rf $BUILD_DIR
	mkdir -p $BUILD_DIR

	go build -ldflags "-X main.Version $VERSION" rollbackup.go	
	strip rollbackup
	
	mkdir -p $BUILD_DIR/usr/local/bin
	cp rollbackup $BUILD_DIR/usr/local/bin/

	mkdir -p $BUILD_DIR/etc/cron.d
	echo "* * * * * root /usr/local/bin/rollbackup backup >> /var/log/rollbackup_cron.log 2>&1" > $BUILD_DIR/etc/cron.d/rollbackup
	chmod 600 $BUILD_DIR/etc/cron.d/rollbackup
	
	fpm -t deb -C $BUILD_DIR -s dir -n rollbackup -f -a $ARCH -v $VERSION --package rollbackup_$ARCH.deb $fpm_flags .
	fpm -t rpm -C $BUILD_DIR -s dir -n rollbackup -f -a $ARCH --epoch 0 -v $VERSION --package rollbackup_$ARCH.rpm $fpm_flags .

	if [ ! -z "$CIRCLE_ARTIFACTS" ]; then
		mv *.deb $CIRCLE_ARTIFACTS/
		mv *.rpm $CIRCLE_ARTIFACTS/
		mkdir -p $CIRCLE_ARTIFACTS/bin/$ARCH/
		cp rollbackup $CIRCLE_ARTIFACTS/bin/$ARCH/rollbackup
	fi
}

cd rollbackup

GOARCH=amd64 ARCH=amd64 _build
GOARCH=386 ARCH=i386 _build
GOARM=6 GOARCH=arm ARCH=armhf _build
