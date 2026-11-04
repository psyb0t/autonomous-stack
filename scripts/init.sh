#!/bin/bash
set -euo pipefail
umask 077
LOG_FILE="${LOG_FILE:-/tmp/autonomous-stack-init.log}"
exec > >(tee -a "$LOG_FILE") 2>&1
cd "$(dirname "${BASH_SOURCE[0]}")/.."
log() {
	printf '{"time":"%s","level":"%s","file":"init.sh","line":%s,"func":"%s","msg":"%s"}\n' \
		"$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1" "${BASH_LINENO[0]}" "${FUNCNAME[1]:-main}" "$2" >&2
}
trap 'log ERROR "Command failed at line $LINENO"' ERR
if [[ $# -ne 0 ]]; then
	log ERROR 'Usage: make init'
	exit 1
fi
if [[ -e .env ]]; then
	log INFO 'Existing .env preserved. Edit it, then run make setup.'
	exit 0
fi
umask 077
env_tmp=$(mktemp .env.init.XXXXXX)
trap 'rm -f "$env_tmp"' EXIT
cp .env.example "$env_tmp"
for key in AUTONOMOUS_TOKEN BROWSER_TOKEN MAILBOX_TOKEN; do
	value=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
	sed -i "s/^${key}=.*/${key}=${value}/" "$env_tmp"
done
sed -i "s/^STACK_UID=.*/STACK_UID=$(id -u)/;s/^STACK_GID=.*/STACK_GID=$(id -g)/" "$env_tmp"
# A hard link publishes the completed file only if another init has not created it.
if ln "$env_tmp" .env; then
	log INFO 'Created private .env. Set Claude, Telegram and mail credentials, then run make setup.'
else
	log WARN 'Another init created .env. That file was preserved.'
fi
