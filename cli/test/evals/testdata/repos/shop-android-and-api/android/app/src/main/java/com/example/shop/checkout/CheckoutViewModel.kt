package com.example.shop.checkout

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

class CheckoutViewModel(
    private val repository: CheckoutRepository = CheckoutRepository(),
) : ViewModel() {
    sealed interface UiState {
        data object Idle : UiState
        data object Placing : UiState
        data class Done(val orderId: String) : UiState
        data class Failed(val message: String) : UiState
    }

    private val _state = MutableStateFlow<UiState>(UiState.Idle)
    val state: StateFlow<UiState> = _state

    fun placeOrder(cartId: String, itemCount: Int, paymentMethodId: String) {
        println("CheckoutViewModel: placing order ($itemCount items)")
        _state.value = UiState.Placing
        viewModelScope.launch {
            when (val result = repository.placeOrder(cartId, paymentMethodId)) {
                is CheckoutRepository.Result.Placed -> _state.value = UiState.Done(result.order.id)
                is CheckoutRepository.Result.Rejected -> _state.value = UiState.Failed(result.message)
            }
        }
    }
}
