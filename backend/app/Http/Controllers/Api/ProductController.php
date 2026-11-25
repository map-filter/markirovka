<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Requests\ProductRequest;
use App\Http\Resources\ProductResource;
use App\Models\Code;
use App\Models\Operation;
use App\Models\Product;
use App\Services\NktService;
use App\Services\ProductImporter;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Http\Response;
use Symfony\Component\HttpFoundation\StreamedResponse;

class ProductController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        $org = $request->user()->organization_id;
        $query = Product::where('organization_id', $org)->orderBy('category')->orderBy('name');
        if ($search = trim((string) $request->query('search'))) {
            $like = '%'.mb_strtolower($search).'%';
            $query->where(fn ($q) => $q->whereRaw('lower(name) like ?', [$like])
                ->orWhere('gtin', 'like', "%{$search}%")
                ->orWhereRaw('lower(model) like ?', [$like]));
        }
        if ($category = $request->query('category')) {
            $query->where('category', $category);
        }
        $page = $query->paginate(min((int) $request->query('per_page', 25), 200));
        Code::attachCounts($page->getCollection(), 'product_id');

        return ProductResource::collection($page);
    }

    public function categories(Request $request): JsonResponse
    {
        $list = Product::where('organization_id', $request->user()->organization_id)
            ->whereNotNull('category')->distinct()->orderBy('category')->pluck('category');

        return response()->json(['data' => $list]);
    }

    public function store(ProductRequest $request): JsonResponse
    {
        $product = Product::create($request->productData() + ['organization_id' => $request->user()->organization_id]);
        Operation::log($product->organization_id, 'product.created', "Добавлен товар {$product->name} ({$product->gtin})", subject: $product);
        Code::attachCounts(collect([$product]), 'product_id');

        return (new ProductResource($product))->response()->setStatusCode(201);
    }

    public function show(Request $request, int $id): ProductResource
    {
        $product = $this->find($request, $id);
        Code::attachCounts(collect([$product]), 'product_id');

        return new ProductResource($product);
    }

    public function update(ProductRequest $request, int $id): ProductResource
    {
        $product = $this->find($request, $id);
        $data = $request->productData();
        if ($data['gtin'] !== $product->gtin) {
            $data['nkt_status'] = Product::NKT_UNCHECKED;
            $data['nkt_checked_at'] = null;
        }
        $product->update($data);
        Operation::log($product->organization_id, 'product.updated', "Изменён товар {$product->name}", subject: $product);
        Code::attachCounts(collect([$product]), 'product_id');

        return new ProductResource($product);
    }

    public function destroy(Request $request, int $id): Response|JsonResponse
    {
        $product = $this->find($request, $id);
        if ($product->codes()->exists()) {
            return response()->json(['message' => 'У товара есть коды маркировки, удалить нельзя'], 409);
        }
        $product->delete();
        Operation::log($product->organization_id, 'product.deleted', "Удалён товар {$product->name}");

        return response()->noContent();
    }

    public function nktCheck(Request $request, int $id, NktService $nkt): JsonResponse
    {
        $product = $this->find($request, $id);
        $card = $nkt->check($product);
        Code::attachCounts(collect([$product]), 'product_id');

        return response()->json(['data' => new ProductResource($product), 'nkt' => $card]);
    }

    public function nktRegister(Request $request, int $id, NktService $nkt): JsonResponse
    {
        $product = $this->find($request, $id);
        if (config('ismpt.mode') !== 'emulator') {
            return response()->json(['message' => 'Карточка создаётся в личном кабинете НКТ, затем нажмите «Проверить в НКТ»'], 409);
        }
        $nkt->register($product);
        Code::attachCounts(collect([$product]), 'product_id');

        return response()->json(['data' => new ProductResource($product)]);
    }

    public function import(Request $request, ProductImporter $importer): JsonResponse
    {
        $request->validate([
            'file' => ['required', 'file', 'max:10240', 'mimes:csv,txt,xlsx'],
        ]);
        $result = $importer->import($request->user()->organization, $request->file('file'));
        Operation::log(
            $request->user()->organization_id,
            'product.imported',
            "Импорт товаров: создано {$result['created']}, обновлено {$result['updated']}, ошибок ".count($result['errors']),
            count($result['errors']) ? 'warning' : 'info',
        );

        return response()->json($result);
    }

    public function template(): StreamedResponse
    {
        $headers = ProductImporter::HEADERS;

        return response()->streamDownload(function () use ($headers) {
            $out = fopen('php://output', 'w');
            fwrite($out, "\xEF\xBB\xBF");
            fputcsv($out, array_values($headers), ';');
            fputcsv($out, ['04870712009990', 'Футболка мужская, белый, р. M', 'FT-M-01', 'M', 'белый', 'хлопок 100%', '6109100000', 'футболки', 'Кендір', 'Казахстан'], ';');
            fclose($out);
        }, 'tovary-shablon.csv', ['Content-Type' => 'text/csv; charset=UTF-8']);
    }

    private function find(Request $request, int $id): Product
    {
        return Product::where('organization_id', $request->user()->organization_id)->findOrFail($id);
    }
}
