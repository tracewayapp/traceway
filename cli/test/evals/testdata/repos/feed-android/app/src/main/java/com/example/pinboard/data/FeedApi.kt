package com.example.pinboard.data

import com.example.pinboard.feed.FeedItem
import retrofit2.http.GET
import retrofit2.http.Path
import retrofit2.http.Query

data class FeedPage(val items: List<FeedItem>, val nextCursor: String?)

interface FeedApi {
    @GET("v2/feed")
    suspend fun feed(@Query("cursor") cursor: String?, @Query("limit") limit: Int = 20): FeedPage

    @GET("v2/posts/{id}")
    suspend fun post(@Path("id") id: String): FeedItem
}
