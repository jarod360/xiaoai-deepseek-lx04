#!/system/bin/sh
# V68 cleanup: no awk dependency; stop only the server named by this project's owned marker.
# Never signal OEM or XiaoSeek Agent. No SIGKILL fallback.
set -f
R=/data/local/tmp/xiaoseek_v53_phase_a
S="$R/frida-server"
F="$R/owned-server.pid"
EXPECTED_SHA=07ba6529925dbcb4a3214a3293b8b31dbbfb0c74efe3c1e4f937ebb46be502ab
OEM_PKG=com.xiaomi.micolauncher
valid_pid() { case "$1" in ''|*[!0-9]*) return 1;; esac; return 0; }
owned_server() {
    p="$1"
    [ -r "/proc/$p/cmdline" ] || return 1
    [ -r "/proc/$p/status" ] || return 1
    grep -q '^State:[[:space:]]*Z' "/proc/$p/status" && return 1
    line=$(tr '\000' ' ' < "/proc/$p/cmdline" 2>/dev/null) || return 1
    case "$line" in "$S -P -l 127.0.0.1:27042"*) return 0;; *) return 1;; esac
}
oem_pids() {
    ps -A 2>/dev/null | while IFS= read -r row; do
        set -- $row
        [ "$#" -ge 2 ] || continue
        candidate="$2"
        valid_pid "$candidate" || continue
        last=''
        for field do last="$field"; done
        [ "$last" = "$OEM_PKG" ] && printf '%s\n' "$candidate"
    done
}
oem_tracer() {
    sed -n 's/^TracerPid:[[:space:]]*//p' "/proc/$1/status" 2>/dev/null
}
case "$1" in
stop)
    expected_oem="$2"
    valid_pid "$expected_oem" || { echo V68_BAD_EXPECTED_OEM_PID_ABORT; exit 2; }
    [ -x "$S" ] || { echo V68_SERVER_BINARY_MISSING_ABORT; exit 3; }
    [ -r "$F" ] || { echo V68_OWNED_MARKER_MISSING_ABORT; exit 4; }
    p=$(cat "$F" 2>/dev/null)
    valid_pid "$p" || { echo V68_BAD_OWNED_MARKER_ABORT; exit 5; }
    actual_oem=$(oem_pids)
    set -- $actual_oem
    [ "$#" -eq 1 ] || { echo V68_OEM_PROCESS_COUNT_ABORT; exit 6; }
    [ "$1" = "$expected_oem" ] || { echo V68_OEM_PID_CHANGED_ABORT; exit 7; }
    tracer=$(oem_tracer "$expected_oem")
    [ "$tracer" = 0 ] || { echo "V68_OEM_TRACED_ABORT tracer=$tracer"; exit 8; }
    hashline=$(sha256sum "$S" 2>/dev/null)
    set -- $hashline
    [ "$1" = "$EXPECTED_SHA" ] || { echo V68_SERVER_BINARY_SHA_MISMATCH_ABORT; exit 9; }
    if owned_server "$p"; then
        echo "V68_OWNED_SERVER_MATCH pid=$p"
        line=$(tr '\000' ' ' < "/proc/$p/cmdline" 2>/dev/null)
        echo "V68_SERVER_CMDLINE=$line"
        echo "V68_OEM_PID=$expected_oem TRACER_PID=0"
        echo "V68_TERM_EXACT_OWNED_SERVER_PID=$p"
        kill -TERM "$p" 2>/dev/null || :
        i=0
        while [ "$i" -lt 8 ]; do
            sleep 1
            if ! owned_server "$p"; then
                if [ -d "/proc/$p" ] && ! grep -q '^State:[[:space:]]*Z' "/proc/$p/status"; then
                    echo V68_SERVER_PID_REUSED_ABORT; exit 10
                fi
                break
            fi
            i=$((i+1))
        done
        if owned_server "$p"; then echo V68_SERVER_STILL_RUNNING_NO_SIGKILL; exit 11; fi
    else
        if [ -d "/proc/$p" ] && ! grep -q '^State:[[:space:]]*Z' "/proc/$p/status"; then
            echo V68_MARKER_PID_IDENTITY_MISMATCH_ABORT; exit 12
        fi
        echo "V68_SERVER_ALREADY_STOPPED pid=$p"
    fi
    actual_oem=$(oem_pids)
    set -- $actual_oem
    [ "$#" -eq 1 ] && [ "$1" = "$expected_oem" ] || { echo V68_OEM_POST_PID_CHANGED_ABORT; exit 13; }
    tracer=$(oem_tracer "$expected_oem")
    [ "$tracer" = 0 ] || { echo "V68_OEM_POST_TRACED_ABORT tracer=$tracer"; exit 14; }
    rm -f "$F" || { echo V68_MARKER_REMOVE_FAILED; exit 15; }
    echo V68_OWNED_SERVER_STOPPED=1
    echo V68_OWNED_MARKER_REMOVED=1
    echo V68_OEM_PROCESS_UNCHANGED=1
    echo V68_CLEANUP_PASS=1
    ;;
*) echo 'usage: stop <expected-oem-pid>'; exit 1;;
esac