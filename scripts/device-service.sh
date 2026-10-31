#!/system/bin/sh
# Magisk service.d entry. No network services are exposed by this client.
/system/bin/sh /data/adb/lx04-deepseek/supervise.sh </dev/null >/dev/null 2>&1 &
