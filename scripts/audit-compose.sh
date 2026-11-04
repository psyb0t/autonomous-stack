#!/bin/bash
set -euo pipefail
log() {
	printf '{"time":"%s","level":"%s","file":"audit-compose.sh","line":%s,"func":"%s","msg":"%s"}\n' \
		"$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1" "${BASH_LINENO[0]}" "${FUNCNAME[1]:-main}" "$2" >&2
}
trap 'log ERROR "Compose audit failed at line $LINENO"' ERR
[[ $# == 1 && -f "$1" ]] || {
	log ERROR 'Pass one rendered Compose JSON file.'
	exit 2
}
# Only these upstream entrypoints have the documented root bootstrap exception.
jq -e '.services | type == "object" and length > 0' "$1" >/dev/null
jq -e '
  def image_ok:
    test("^(docker.io/)?psyb0t/[^:@]+:latest$") or
    test("@sha256:[a-f0-9]{64}$");
  [.services | to_entries[] |
    .key as $name | .value as $s |
    if $name == "prepare" then
      ($s.network_mode == "none" and $s.read_only == true and
       ($s.cap_drop | index("ALL")) != null)
    else
      (($s.image | image_ok) and
       $s.init == true and
       ($s.cap_drop | index("ALL")) != null and
       ($s.security_opt | index("no-new-privileges:true")) != null and
       $s.privileged != true and $s.network_mode != "host" and
       ($s.devices // [] | length) == 0 and $s.networks != null and
       $s.pid != "host" and $s.ipc != "host" and
       ($s.mem_limit | tonumber) > 0 and ($s.cpus | tonumber) > 0 and
       $s.pids_limit > 0 and $s.healthcheck.test != null and
       $s.logging.options["max-size"] != null and
       $s.logging.options["max-file"] != null and
       $s.restart == "unless-stopped" and
       all($s.ports[]?; .host_ip == "127.0.0.1" or .host_ip == "::1") and
       all($s.volumes[]?;
         .type != "bind" or
         ((.source | test("(^/$|^/etc$|^/var$|^/root$|^/home(/[^/]+)?$|docker.sock$)")) | not)) and
       (if $name == "browser" or $name == "claude" then
          ($s.cap_add | sort) == ["CHOWN","DAC_OVERRIDE","FOWNER","SETGID","SETUID"]
        else
          $s.read_only == true and ($s.cap_add // [] | length) == 0
        end))
    end
  ] | all
' "$1" >/dev/null
log INFO 'Compose policy passed, including the documented bootstrap exceptions.'
