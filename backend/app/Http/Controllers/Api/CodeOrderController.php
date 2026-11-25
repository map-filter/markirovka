<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\CodeOrderResource;
use App\Models\Code;
use App\Models\CodeOrder;
use App\Models\Product;
use App\Services\CodeOrderService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Validation\Rule;

class CodeOrderController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        $request->validate(['status' => ['nullable', Rule::in([CodeOrder::NEW, CodeOrder::SENT, CodeOrder::PROCESSING, CodeOrder::READY, CodeOrder::FAILED])]]);
        $query = CodeOrder::with('product')->where('organization_id', $request->user()->organization_id)->latest('id');
        if ($status = $request->query('status')) {
            $query->where('status', $status);
        }
        $page = $query->paginate(25);
        Code::attachCounts($page->getCollection(), 'code_order_id');

        return CodeOrderResource::collection($page);
    }

    public function store(Request $request, CodeOrderService $service): JsonResponse
    {
        $data = $request->validate([
            'product_id' => ['required', 'integer'],
            'quantity' => ['required', 'integer', 'min:1', 'max:10000'],
        ]);
        $product = Product::where('organization_id', $request->user()->organization_id)->findOrFail($data['product_id']);
        $order = $service->create($request->user(), $product, $data['quantity']);
        Code::attachCounts(collect([$order]), 'code_order_id');

        return (new CodeOrderResource($order->load('product')))->response()->setStatusCode(201);
    }

    public function show(Request $request, int $id): CodeOrderResource
    {
        $order = $this->find($request, $id);
        Code::attachCounts(collect([$order]), 'code_order_id');

        return new CodeOrderResource($order);
    }

    public function refresh(Request $request, int $id, CodeOrderService $service): CodeOrderResource
    {
        $order = $this->find($request, $id);
        $service->process($order);
        $order->refresh()->load('product');
        Code::attachCounts(collect([$order]), 'code_order_id');

        return new CodeOrderResource($order);
    }

    private function find(Request $request, int $id): CodeOrder
    {
        return CodeOrder::with('product')->where('organization_id', $request->user()->organization_id)->findOrFail($id);
    }
}
