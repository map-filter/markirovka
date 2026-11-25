<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\CodeResource;
use App\Http\Resources\ProductShortResource;
use App\Models\Code;
use App\Models\Product;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Validation\Rule;

class CodeController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        $request->validate(['status' => ['nullable', Rule::in(Code::STATUSES)]]);
        $query = Code::with(['product', 'codeOrder'])
            ->where('organization_id', $request->user()->organization_id)
            ->orderByDesc('id');
        foreach (['status' => 'status', 'product_id' => 'product_id', 'order_id' => 'code_order_id'] as $param => $column) {
            if ($value = $request->query($param)) {
                $query->where($column, $value);
            }
        }
        if ($search = trim((string) $request->query('search'))) {
            $query->where('cis', 'like', '%'.str_replace(['%', '_'], ['\%', '\_'], $search).'%');
        }

        return CodeResource::collection($query->paginate(min((int) $request->query('per_page', 50), 500)));
    }

    public function summary(Request $request): JsonResponse
    {
        $org = $request->user()->organization_id;
        $products = Product::where('organization_id', $org)->whereHas('codes')->orderBy('name')->get();
        Code::attachCounts($products, 'product_id');

        return response()->json([
            'data' => $products->map(fn (Product $p) => [
                'product' => new ProductShortResource($p),
                'counts' => $p->getAttribute('code_counts'),
            ]),
        ]);
    }
}
