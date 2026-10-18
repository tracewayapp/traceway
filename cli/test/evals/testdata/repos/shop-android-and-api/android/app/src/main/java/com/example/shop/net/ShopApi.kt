package com.example.shop.net

import retrofit2.Call
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

data class CartDto(val id: String, val items: List<CartItemDto>, val totalCents: Long)
data class CartItemDto(val sku: String, val qty: Int, val unitPriceCents: Long)
data class CheckoutRequest(val cartId: String, val paymentMethodId: String)
data class OrderDto(val id: String, val status: String, val totalCents: Long)

interface ShopApi {
    @GET("api/cart")
    fun cart(): Call<CartDto>

    @POST("api/checkout")
    fun checkout(@Body body: CheckoutRequest): Call<OrderDto>
}
