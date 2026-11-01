package top.xiaoseek.probe;

import android.content.BroadcastReceiver;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.ServiceConnection;
import android.os.Binder;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.Parcel;
import android.os.RemoteException;
import android.os.SystemClock;
import android.util.Log;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Read-only OEM Binder reachability test. Never sends ai_service/stopTts. */
public final class ProbeReceiver extends BroadcastReceiver {
    private static final String TAG = "XiaoSeekBinderV40";
    private static final String LOG_FILE = "binder-v40.log";
    private static final Object FILE_LOCK = new Object();

    // APK-private file: independent of logd/logcat. Records no identifiers or callback payload.
    private static void report(Context context, String message) {
        Log.i(TAG, message);
        synchronized (FILE_LOCK) {
            try (FileOutputStream out = context.openFileOutput(LOG_FILE, Context.MODE_APPEND)) {
                out.write((System.currentTimeMillis() + " " + message + "\n").getBytes(StandardCharsets.UTF_8));
            } catch (IOException e) {
                Log.e(TAG, "local-log-write-failed", e);
            }
        }
    }
    private static final String SERVICE = "com.xiaomi.micolauncher.common.services.deviceservice.MicoDeviceService";
    private static final String DESCRIPTOR = "com.xiaomi.mico.device.service.iface.IMicoDeviceServiceInterface";
    private static final String CALLBACK = "com.xiaomi.mico.device.service.iface.IUbusRelayCallback";

    @Override public void onReceive(Context receiverContext, Intent ignored) {
        final PendingResult pending = goAsync();
        // BroadcastReceiver's restricted Context cannot directly bind services.
        final Context context = receiverContext.getApplicationContext();
        report(context, "receiver_entered");
        final long started = SystemClock.elapsedRealtime();
        final AtomicBoolean finished = new AtomicBoolean(false);
        final Handler handler = new Handler(Looper.getMainLooper());
        final boolean[] bound = {false};
        final ServiceConnection[] connection = new ServiceConnection[1];

        final Runnable cleanup = () -> {
            if (!finished.compareAndSet(false, true)) return;
            if (bound[0]) {
                try { context.unbindService(connection[0]); }
                catch (RuntimeException e) { report(context, "unbind_failed=" + e); }
            }
            pending.finish();
        };
        handler.postDelayed(() -> {
            if (!finished.get()) report(context, "timeout_after_ms=5000");
            cleanup.run();
        }, 5000);

        connection[0] = new ServiceConnection() {
            @Override public void onServiceConnected(ComponentName name, IBinder service) {
                if (finished.get()) return;
                report(context, "bound_ms=" + (SystemClock.elapsedRealtime() - started));
                new Thread(() -> {
                    Parcel request = Parcel.obtain();
                    Parcel response = Parcel.obtain();
                    try {
                        if (!DESCRIPTOR.equals(service.getInterfaceDescriptor()))
                            throw new SecurityException("unexpected Binder descriptor");
                        // Independent synchronous read-only API from the same OEM Binder.
                        // The result does not depend on UBus or a callback.
                        Parcel romReq = Parcel.obtain();
                        Parcel romReply = Parcel.obtain();
                        try {
                            romReq.writeInterfaceToken(DESCRIPTOR);
                            long romStarted = SystemClock.elapsedRealtime();
                            if (!service.transact(4, romReq, romReply, 0))
                                throw new RemoteException("getROMVersion transaction 4 rejected");
                            romReply.readException();
                            String rom = romReply.readString();
                            report(context, "rom_query=ok elapsed_ms=" +
                                    (SystemClock.elapsedRealtime() - romStarted) +
                                    " nonempty=" + (rom != null && rom.length() > 0));
                        } catch (Exception romErr) {
                            report(context, "rom_query=error exception=" + romErr);
                        } finally {
                            romReply.recycle();
                            romReq.recycle();
                        }
                        request.writeInterfaceToken(DESCRIPTOR);
                        request.writeString("mibrain");
                        request.writeString("tts_vendor_show");
                        request.writeString("{}");
                        request.writeStrongBinder(new Binder() {
                            { attachInterface(null, CALLBACK); }
                            @Override protected boolean onTransact(int code, Parcel data, Parcel reply, int flags)
                                    throws RemoteException {
                                report(context, "callback_entered code=" + code + " flags=" + flags);
                                if (code == IBinder.INTERFACE_TRANSACTION) {
                                    reply.writeString(CALLBACK);
                                    return true;
                                }
                                if (code != IBinder.FIRST_CALL_TRANSACTION) return super.onTransact(code, data, reply, flags);
                                data.enforceInterface(CALLBACK);
                                String result = data.readString();
                                Matcher matcher = Pattern.compile("\\\"code\\\"\\s*:\\s*(-?\\d+)").matcher(result == null ? "" : result);
                                String status = matcher.find() ? matcher.group(1) : "unknown";
                                report(context, "callback_code=" + status + " elapsed_ms=" +
                                        (SystemClock.elapsedRealtime() - started));
                                if (reply != null) reply.writeNoException();
                                handler.post(cleanup);
                                return true;
                            }
                        });
                        // Stock DEX: IMicoDeviceServiceInterface.Stub.onTransact code 12 = ubusRelay.
                        boolean sent = service.transact(12, request, response, 0);
                        if (!sent) throw new RemoteException("transaction 12 not accepted");
                        response.readException();
                        report(context, "transact_complete_ms=" + (SystemClock.elapsedRealtime() - started));
                    } catch (Exception e) {
                        report(context, "bind_or_transact_failed_ms=" + (SystemClock.elapsedRealtime() - started) + " exception=" + e);
                        handler.post(cleanup);
                    } finally {
                        response.recycle();
                        request.recycle();
                    }
                }, "XiaoSeekBinderProbe").start();
            }
            @Override public void onServiceDisconnected(ComponentName name) {
                report(context, "oem_service_disconnected");
                handler.post(cleanup);
            }
        };
        try {
            Intent service = new Intent().setComponent(new ComponentName("com.xiaomi.micolauncher", SERVICE));
            bound[0] = context.bindService(service, connection[0], Context.BIND_AUTO_CREATE);
            report(context, "bind_requested=" + bound[0]);
            if (!bound[0]) handler.post(cleanup);
        } catch (Exception e) {
            report(context, "bind_rejected=" + e);
            handler.post(cleanup);
        }
    }
}
