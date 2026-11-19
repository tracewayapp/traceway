package com.example.pinboard

import android.app.Application
import com.example.pinboard.data.FeedApi
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions
import com.tracewayapp.traceway.network.TracewayOkHttpInterceptor
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

class PinboardApp : Application() {
    companion object {
        lateinit var http: OkHttpClient
        lateinit var api: FeedApi
    }

    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            application = this,
            connectionString = BuildConfig.TRACEWAY_DSN,
            options = TracewayOptions(version = BuildConfig.VERSION_NAME, proguardUuid = BuildConfig.TRACEWAY_PROGUARD_UUID),
        )
        http = OkHttpClient.Builder().addInterceptor(TracewayOkHttpInterceptor()).build()
        api = Retrofit.Builder()
            .baseUrl("https://api.pinboard.example/")
            .client(http)
            .addConverterFactory(GsonConverterFactory.create())
            .build()
            .create(FeedApi::class.java)
    }
}
