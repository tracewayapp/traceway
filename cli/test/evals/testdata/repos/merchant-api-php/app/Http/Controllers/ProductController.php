<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class ProductController extends Controller
{
    public function index(int $store): JsonResponse
    {
        return response()->json(
            DB::table('products')
                ->select('products.id', 'products.sku', 'products.name', 'products.price', 'products.category_id')
                ->where('products.store_id', $store)
                ->orderBy('products.name')
                ->forPage(1, 50)
                ->get()
        );
    }

    public function update(Request $request, int $store, int $product): JsonResponse
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:200'],
            'price' => ['required', 'numeric', 'min:0'],
        ]);

        DB::table('products')
            ->where('id', $product)
            ->update([...$data, 'updated_at' => now()]);

        return response()->json(['id' => $product]);
    }
}
