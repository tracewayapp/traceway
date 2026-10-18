package com.example.shop.net

import com.example.shop.BuildConfig
import com.tracewayapp.traceway.network.TracewayOkHttpInterceptor
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.converter.moshi.MoshiConverterFactory

object ApiClient {
    // OkHttp's defaults: 10s connect, 10s read, 10s write.
    private val http = OkHttpClient.Builder()
        .addInterceptor(AuthInterceptor())
        .addInterceptor(TracewayOkHttpInterceptor())
        .build()

    val retrofit: Retrofit = Retrofit.Builder()
        .baseUrl(BuildConfig.API_BASE_URL)
        .client(http)
        .addConverterFactory(MoshiConverterFactory.create())
        .build()

    val shop: ShopApi = retrofit.create(ShopApi::class.java)
}
