package com.wails.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;

import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;

/**
 * A minimal started foreground service. It does no work of its own — its purpose
 * is to keep the app's process alive (with the required ongoing notification) so
 * the developer's Go goroutines keep running while the app is backgrounded,
 * which Android would otherwise be free to kill. Start it from
 * {@link WailsBridge#startForegroundService(String)} and stop it with
 * {@link WailsBridge#stopForegroundService()}.
 */
public class WailsForegroundService extends android.app.Service {
    public static final String ACTION_START = "com.wails.app.FGS_START";
    private static final String CHANNEL_ID = "wails_foreground";
    public static final int NOTIFICATION_ID = 0x57A1; // "WAI"

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String title = "Wails";
        String text = "Running in the background";
        if (intent != null) {
            if (intent.getStringExtra("title") != null) title = intent.getStringExtra("title");
            if (intent.getStringExtra("text") != null) text = intent.getStringExtra("text");
        }

        NotificationManager nm = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            // The badge IS the unread counter: Samsung-style launchers badge
            // the icon with this notification's number, so the channel must
            // allow badges (new_mail channel separately badges too). Channels
            // ignore most changes after creation — recreate once if an old
            // channel blocked badges.
            NotificationChannel ch = nm.getNotificationChannel(CHANNEL_ID);
            if (ch != null && !ch.canShowBadge()) {
                nm.deleteNotificationChannel(CHANNEL_ID);
                ch = null;
            }
            if (ch == null) {
                ch = new NotificationChannel(CHANNEL_ID, "Background work",
                        NotificationManager.IMPORTANCE_MIN);
                ch.setDescription("Keeps BOI checking for new mail; carries the unread-count badge");
                ch.setShowBadge(true);
                nm.createNotificationChannel(ch);
            }
        }

        PendingIntent contentIntent = null;
        Intent launch = getPackageManager().getLaunchIntentForPackage(getPackageName());
        if (launch != null) {
            int piFlags = Build.VERSION.SDK_INT >= Build.VERSION_CODES.M
                    ? PendingIntent.FLAG_IMMUTABLE : 0;
            contentIntent = PendingIntent.getActivity(this, 0, launch, piFlags);
        }

        Notification n = new NotificationCompat.Builder(this, CHANNEL_ID)
                .setSmallIcon(android.R.drawable.ic_popup_sync)
                .setContentTitle(title)
                .setContentText(text)
                .setOngoing(true)
                .setContentIntent(contentIntent)
                .build();

        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(NOTIFICATION_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
            } else {
                startForeground(NOTIFICATION_ID, n);
            }
        } catch (Exception e) {
            // Android 12+ forbids startForeground() when the app is truly
            // backgrounded — which is exactly what a START_STICKY restart is
            // (system re-creates us after Doze killed the process). Throwing
            // here is a FATAL main-thread crash: the whole process — Go poll
            // loop included — dies and background mail silently stops.
            // Bow out instead; the next MainActivity.onCreate restarts us from
            // the foreground, where the call is allowed.
            android.util.Log.w("WailsForegroundService", "startForeground rejected (background restart): " + e);
            stopSelf();
            return START_NOT_STICKY;
        }
        // Restart if the OS kills us while still wanted.
        return START_STICKY;
    }

    @Nullable
    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
