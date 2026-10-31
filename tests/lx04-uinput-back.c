// SPDX-License-Identifier: Apache-2.0
// Standalone LX04 uinput BACK latency test; does not modify XiaoSeek.
#include <errno.h>
#include <fcntl.h>
#include <linux/input.h>
#include <linux/uinput.h>
#include <stdio.h>
#include <string.h>
#include <sys/ioctl.h>
#include <time.h>
#include <unistd.h>

static long millis(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (long)t.tv_sec * 1000 + t.tv_nsec / 1000000;
}
static int original_overlay_focused(void) {
    FILE *fp = popen("/system/bin/dumpsys window windows", "r");
    if (!fp) return 0;
    char line[2048];
    int matched = 0;
    while (fgets(line, sizeof(line), fp)) {
        if (strstr(line, "mCurrentFocus=") &&
            strstr(line, "Sys2038:com.xiaomi.micolauncher/com.xiaomi.micolauncher.Launcher")) {
            matched = 1;
        }
    }
    if (pclose(fp) != 0) return 0;
    return matched;
}
static int emit(int fd, unsigned short type, unsigned short code, int value) {
    struct input_event event = {0};
    event.type = type;
    event.code = code;
    event.value = value;
    return write(fd, &event, sizeof(event)) == sizeof(event) ? 0 : -1;
}
int main(void) {
    long start = millis();
    if (!original_overlay_focused()) {
        fprintf(stderr, "SKIP: original Sys2038 overlay is not focused; no BACK sent\n");
        return 2;
    }
    int fd = open("/dev/uinput", O_WRONLY | O_NONBLOCK);
    if (fd < 0) { perror("open /dev/uinput"); return 1; }
    if (ioctl(fd, UI_SET_EVBIT, EV_KEY) < 0 ||
        ioctl(fd, UI_SET_KEYBIT, KEY_BACK) < 0) {
        perror("uinput key setup"); close(fd); return 1;
    }
    struct uinput_user_dev dev = {0};
    snprintf(dev.name, sizeof(dev.name), "LX04 BACK test");
    dev.id.bustype = BUS_VIRTUAL;
    dev.id.vendor = 0x1234;
    dev.id.product = 0x0001;
    dev.id.version = 1;
    if (write(fd, &dev, sizeof(dev)) != sizeof(dev) ||
        ioctl(fd, UI_DEV_CREATE) < 0) {
        perror("uinput create"); close(fd); return 1;
    }
    // Give Android InputReader time to register the virtual keyboard.
    sleep(1);
    if (!original_overlay_focused()) {
        fprintf(stderr, "SKIP: focus changed; no BACK sent\n");
        ioctl(fd, UI_DEV_DESTROY); close(fd); return 2;
    }
    long before = millis();
    int ok = emit(fd, EV_KEY, KEY_BACK, 1) ||
             emit(fd, EV_SYN, SYN_REPORT, 0);
    usleep(40000);
    ok |= emit(fd, EV_KEY, KEY_BACK, 0) ||
          emit(fd, EV_SYN, SYN_REPORT, 0);
    printf("uinput BACK injection: %ld ms (total %ld ms)\n", millis()-before, millis()-start);
    usleep(200000);
    ioctl(fd, UI_DEV_DESTROY);
    close(fd);
    return ok ? 1 : 0;
}
