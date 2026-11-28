package com.example.pinboard

import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class PinboardApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            this,
            TracewayOptions(
                token = BuildConfig.TRACEWAY_TOKEN,
                baseUrl = BuildConfig.TRACEWAY_URL,
                screenCapture = true,
            ),
        )
    }
}
