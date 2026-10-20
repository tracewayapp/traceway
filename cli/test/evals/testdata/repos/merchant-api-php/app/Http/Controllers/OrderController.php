<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
use Illuminate\Support\Facades\DB;

class OrderController extends Controller
{
    public function index(int $store): JsonResponse
    {
        return response()->json(
            DB::table('orders')
                ->select('orders.id', 'orders.number', 'orders.status', 'orders.total', 'orders.customer_id', 'orders.created_at')
                ->where('orders.store_id', $store)
                ->orderByDesc('orders.created_at')
                ->paginate(25)
        );
    }

    public function show(int $store, int $order): JsonResponse
    {
        $row = DB::table('orders')
            ->select('orders.id', 'orders.number', 'orders.status', 'orders.total', 'orders.customer_id', 'orders.created_at')
            ->where('orders.id', $order)
            ->where('orders.store_id', $store)
            ->firstOrFail();
        $items = DB::table('order_items')
            ->select('order_items.id', 'order_items.product_id', 'order_items.quantity', 'order_items.unit_price')
            ->where('order_items.order_id', $row->id)
            ->get();

        return response()->json(['order' => $row, 'items' => $items]);
    }
}
