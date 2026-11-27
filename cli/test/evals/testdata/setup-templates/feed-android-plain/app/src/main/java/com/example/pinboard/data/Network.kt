package com.example.pinboard.data

import com.example.pinboard.BuildConfig
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

object Network {
    val http: OkHttpClient by lazy { OkHttpClient.Builder().build() }

    val api: FeedApi by lazy {
        Retrofit.Builder()
            .baseUrl(BuildConfig.API_BASE_URL)
            .client(http)
            .addConverterFactory(GsonConverterFactory.create())
            .build()
            .create(FeedApi::class.java)
    }
}
