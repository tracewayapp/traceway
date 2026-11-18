<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class RevenueReportController extends Controller
{
    public function show(Request $request, int $store): JsonResponse
    {
        $store = DB::table('stores')
            ->select('stores.id', 'stores.name', 'stores.currency')
            ->where('stores.id', $store)
            ->where('stores.merchant_id', $request->user()->merchant_id)
            ->firstOrFail();

        // Revenue and distinct buyers per day and category over the last 90 days.
        $rows = DB::table('orders')
            ->join('order_items', 'order_items.order_id', '=', 'orders.id')
            ->join('products', 'products.id', '=', 'order_items.product_id')
            ->where('orders.store_id', $store->id)
            ->where('orders.status', 'paid')
            ->where('orders.created_at', '>=', now()->subDays(90)->startOfDay())
            ->selectRaw('date(orders.created_at) as day, products.category_id, count(distinct orders.customer_id) as buyers, sum(order_items.quantity * order_items.unit_price) as revenue')
            ->groupBy('day', 'products.category_id')
            ->orderBy('day')
            ->get();

        return response()->json([
            'store' => $store,
            'currency' => $store->currency,
            'series' => $rows->groupBy('day')->map(fn ($day) => $day->keyBy('category_id')),
        ]);
    }
}
