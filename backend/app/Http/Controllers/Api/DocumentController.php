<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\DocumentResource;
use App\Models\Document;
use App\Services\DocumentService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;

class DocumentController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        return DocumentResource::collection(
            Document::where('organization_id', $request->user()->organization_id)->latest('id')->paginate(20)
        );
    }

    public function prepare(Request $request, DocumentService $service): JsonResponse
    {
        $data = $request->validate([
            'type' => ['required', 'in:application,introduction'],
            'code_ids' => ['nullable', 'array', 'max:10000'],
            'code_ids.*' => ['integer'],
            'product_id' => ['nullable', 'integer'],
            'order_id' => ['nullable', 'integer'],
        ]);
        $document = $service->prepare($request->user(), $data['type'], $data);

        return (new DocumentResource($document))->response()->setStatusCode(201);
    }

    public function show(Request $request, int $id): DocumentResource
    {
        return new DocumentResource($this->find($request, $id));
    }

    public function submit(Request $request, int $id, DocumentService $service): DocumentResource
    {
        $data = $request->validate(['signature' => ['required', 'string']]);

        return new DocumentResource($service->submit($this->find($request, $id), $data['signature']));
    }

    private function find(Request $request, int $id): Document
    {
        return Document::where('organization_id', $request->user()->organization_id)->findOrFail($id);
    }
}
