package com.example.shop.checkout

import com.example.shop.net.ApiClient
import com.example.shop.net.CheckoutRequest
import com.example.shop.net.OrderDto
import com.example.shop.net.ShopApi
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import retrofit2.HttpException

/**
 * Places orders. The call is blocking (Call.execute) on the IO dispatcher so
 * the response can be mapped in one place.
 */
class CheckoutRepository(
    private val api: ShopApi = ApiClient.shop,
) {
    sealed interface Result {
        data class Placed(val order: OrderDto) : Result
        data class Rejected(val code: Int, val message: String) : Result
    }

    /**
     * POST /api/checkout. Returns Placed or Rejected; HttpException for
     * other HTTP failures.
     */
    suspend fun placeOrder(cartId: String, paymentMethodId: String): Result =
        withContext(Dispatchers.IO) {
            val response = api
                .checkout(CheckoutRequest(cartId, paymentMethodId))
                .execute()
            when {
                response.isSuccessful -> Result.Placed(response.body()!!)
                response.code() == 409 || response.code() == 422 ->
                    Result.Rejected(response.code(), response.errorBody()?.string().orEmpty())
                else -> throw HttpException(response)
            }
        }
}
