package com.example.shop

import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class ShopApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            application = this,
            connectionString = BuildConfig.TRACEWAY_CONNECTION_STRING,
            options = TracewayOptions(
                version = BuildConfig.VERSION_NAME,
                proguardUuid = BuildConfig.TRACEWAY_PROGUARD_UUID,
                // Keep the whole cart-to-checkout flow with a crash, not only its last 10s.
                eventsWindowMs = 120_000,
            ),
        )
    }
}
