#!/bin/bash

set -e

echo "Migrating Database..."
/app/scanner -c /app/configs/config.yml migrate

exec /app/scanner -c /app/configs/config.yml api
