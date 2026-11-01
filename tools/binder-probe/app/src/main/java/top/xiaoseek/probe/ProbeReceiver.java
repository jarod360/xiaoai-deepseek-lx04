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
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Read-only OEM Binder reachability test. Never sends ai_service/stopTts. */
public final class ProbeReceiver extends BroadcastReceiver {
    private static final String TAG = "XiaoSeekBinderV38";
    private static final String SERVICE = "com.xiaomi.micolauncher.common.services.deviceservice.MicoDeviceService";
    private static final String DESCRIPTOR = "com.xiaomi.mico.device.service.iface.IMicoDeviceServiceInterface";
    private static final String CALLBACK = "com.xiaomi.mico.device.service.iface.IUbusRelayCallback";

    @Override public void onReceive(Context receiverContext, Intent ignored) {
        final PendingResult pending = goAsync();
        // BroadcastReceiver's restricted Context cannot directly bind services.
        final Context context = receiverContext.getApplicationContext();
        final long started = SystemClock.elapsedRealtime();
        final AtomicBoolean finished = new AtomicBoolean(false);
        final Handler handler = new Handler(Looper.getMainLooper());
        final boolean[] bound = {false};
        final ServiceConnection[] connection = new ServiceConnection[1];

        final Runnable cleanup = () -> {
            if (!finished.compareAndSet(false, true)) return;
            if (bound[0]) {
                try { context.unbindService(connection[0]); }
                catch (RuntimeException e) { Log.w(TAG, "unbind failed", e); }
            }
            pending.finish();
        };
        handler.postDelayed(() -> {
            if (!finished.get()) Log.e(TAG, "timeout after 5000ms");
            cleanup.run();
        }, 5000);

        connection[0] = new ServiceConnection() {
            @Override public void onServiceConnected(ComponentName name, IBinder service) {
                if (finished.get()) return;
                Log.i(TAG, "bound_ms=" + (SystemClock.elapsedRealtime() - started));
                new Thread(() -> {
                    Parcel request = Parcel.obtain();
                    Parcel response = Parcel.obtain();
                    try {
                        if (!DESCRIPTOR.equals(service.getInterfaceDescriptor()))
                            throw new SecurityException("unexpected Binder descriptor");
                        request.writeInterfaceToken(DESCRIPTOR);
                        request.writeString("mibrain");
                        request.writeString("tts_vendor_show");
                        request.writeString("{}");
                        request.writeStrongBinder(new Binder() {
                            { attachInterface(null, CALLBACK); }
                            @Override protected boolean onTransact(int code, Parcel data, Parcel reply, int flags)
                                    throws RemoteException {
                                if (code == IBinder.INTERFACE_TRANSACTION) {
                                    reply.writeString(CALLBACK);
                                    return true;
                                }
                                if (code != IBinder.FIRST_CALL_TRANSACTION) return super.onTransact(code, data, reply, flags);
                                data.enforceInterface(CALLBACK);
                                String result = data.readString();
                                Matcher matcher = Pattern.compile("\\\"code\\\"\\s*:\\s*(-?\\d+)").matcher(result == null ? "" : result);
                                String status = matcher.find() ? matcher.group(1) : "unknown";
                                Log.i(TAG, "callback_code=" + status + " elapsed_ms=" +
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
                        Log.i(TAG, "transact_complete_ms=" + (SystemClock.elapsedRealtime() - started));
                    } catch (Exception e) {
                        Log.e(TAG, "bind/transact failed after " + (SystemClock.elapsedRealtime() - started) + "ms", e);
                        handler.post(cleanup);
                    } finally {
                        response.recycle();
                        request.recycle();
                    }
                }, "XiaoSeekBinderProbe").start();
            }
            @Override public void onServiceDisconnected(ComponentName name) {
                Log.w(TAG, "OEM service disconnected");
                handler.post(cleanup);
            }
        };
        try {
            Intent service = new Intent().setComponent(new ComponentName("com.xiaomi.micolauncher", SERVICE));
            bound[0] = context.bindService(service, connection[0], Context.BIND_AUTO_CREATE);
            Log.i(TAG, "bind_requested=" + bound[0]);
            if (!bound[0]) handler.post(cleanup);
        } catch (Exception e) {
            Log.e(TAG, "bind rejected", e);
            handler.post(cleanup);
        }
    }
}
