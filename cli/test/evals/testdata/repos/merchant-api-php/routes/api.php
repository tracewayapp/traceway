<?php

use App\Http\Controllers\OrderController;
use App\Http\Controllers\ProductController;
use App\Http\Controllers\RevenueReportController;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;

Route::middleware('auth:sanctum')->group(function () {
    Route::get('/user', fn (Request $request) => $request->user());

    Route::get('/stores/{store}/orders', [OrderController::class, 'index']);
    Route::get('/stores/{store}/orders/{order}', [OrderController::class, 'show']);
    Route::get('/stores/{store}/products', [ProductController::class, 'index']);
    Route::put('/stores/{store}/products/{product}', [ProductController::class, 'update']);

    // The dashboard's revenue chart: last 90 days by day and category.
    Route::get('/stores/{store}/reports/revenue', [RevenueReportController::class, 'show']);
});
