package com.example.pinboard.feed

/**
 * One post of the home feed, as GET /v2/feed returns it (parsed by Gson).
 * Text posts carry no "image" object. Gson does not enforce Kotlin
 * nullability, so anything the API may omit must be declared nullable.
 */
data class FeedItem(
    val id: String,
    val author: Author,
    val text: String,
    val image: ImageRef?,
    val likes: Int,
    val createdAt: String,
)

data class Author(val id: String, val name: String, val avatarUrl: String?)

data class ImageRef(val url: String, val width: Int, val height: Int)
