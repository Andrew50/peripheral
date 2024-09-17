#!/bin/bash
cron
docker-entrypoint.sh postgres -c config_file=/etc/postgresql/postgresql.conf
