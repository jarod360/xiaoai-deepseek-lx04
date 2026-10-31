#!/system/bin/sh
# Keep the installed client alive after native logcat EOF; retain audio recovery.
set -eu
umask 077
APP=/data/adb/lx04-deepseek
LOCK="$APP/supervisor.lock"
if ! mkdir "$LOCK" 2>/dev/null; then
 oldpid=$(cat "$LOCK/pid" 2>/dev/null || true)
 case "$oldpid" in
 ''|*[!0-9]*) ;;
 *) if [ -r "/proc/$oldpid/cmdline" ] && tr '\000' ' ' < "/proc/$oldpid/cmdline" | grep -q "$APP/supervise.sh"; then exit 0; fi ;;
 esac
 rm -f "$LOCK/pid"
 rmdir "$LOCK" 2>/dev/null || exit 1
 mkdir "$LOCK" || exit 1
fi
echo $$ > "$LOCK/pid"
cleanup() { rm -f "$LOCK/pid"; rmdir "$LOCK" 2>/dev/null || true; }
trap cleanup EXIT
delay=3
while [ ! -f "$APP/disabled" ]; do
 started=$(date +%s)
 /system/bin/sh "$APP/run.sh" || true
 [ ! -f "$APP/disabled" ] || break
 finished=$(date +%s)
 if [ "$((finished - started))" -ge 60 ]; then delay=3; fi
 sleep "$delay"
 if [ "$delay" -lt 60 ]; then delay=$((delay * 2)); fi
 if [ "$delay" -gt 60 ]; then delay=60; fi
done