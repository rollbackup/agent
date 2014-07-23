#!/usr/bin/env bash

mkdir -p /etc/rollbackup
id -u rollbackup &>/dev/null || useradd --shell=/sbin/nologin rollbackup
chown rollbackup:rollbackup /etc/rollbackup
chmod 755 /etc/rollbackup 
