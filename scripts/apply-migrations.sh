#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ -f ".env" ]]; then
  while IFS='=' read -r key value; do
    [[ -z "${key}" ]] && continue
    [[ "${key}" =~ ^[[:space:]]*# ]] && continue
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"
    value="${value:-}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    if [[ -z "${!key:-}" ]]; then
      export "${key}=${value}"
    fi
  done < .env
fi

required_vars=(
  PRICEALERT_DB_HOST
  PRICEALERT_DB_PORT
  PRICEALERT_DB_USER
  PRICEALERT_DB_PASSWORD
  PRICEALERT_DB_NAME
)

missing=()
for name in "${required_vars[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    missing+=("$name")
  fi
done

if (( ${#missing[@]} > 0 )); then
  printf 'Missing required environment variables:\n' >&2
  for name in "${missing[@]}"; do
    printf '  - %s\n' "$name" >&2
  done
  printf '\nCopy .env.example to .env and adjust the values, or export them before running this script.\n' >&2
  exit 1
fi

mariadb \
  -h"${PRICEALERT_DB_HOST}" \
  -P"${PRICEALERT_DB_PORT}" \
  -u"${PRICEALERT_DB_USER}" \
  -p"${PRICEALERT_DB_PASSWORD}" \
  "${PRICEALERT_DB_NAME}" < migrations/001_init.sql

mariadb \
  -h"${PRICEALERT_DB_HOST}" \
  -P"${PRICEALERT_DB_PORT}" \
  -u"${PRICEALERT_DB_USER}" \
  -p"${PRICEALERT_DB_PASSWORD}" \
  "${PRICEALERT_DB_NAME}" < migrations/002_price_points.sql

printf 'Applied migrations to database %s\n' "${PRICEALERT_DB_NAME}"
