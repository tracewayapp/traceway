package com.example.shop.net

import com.example.shop.session.SessionStore
import okhttp3.Interceptor
import okhttp3.Response

/** Adds the signed-in user's bearer token to every API call. */
class AuthInterceptor : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val token = SessionStore.accessToken()
        val request = if (token == null) {
            chain.request()
        } else {
            chain.request().newBuilder()
                .header("Authorization", "Bearer $token")
                .build()
        }
        return chain.proceed(request)
    }
}
