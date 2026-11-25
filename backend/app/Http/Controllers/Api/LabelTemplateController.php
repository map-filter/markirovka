<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\LabelTemplateResource;
use App\Models\Code;
use App\Models\LabelTemplate;
use App\Models\Product;
use App\Services\PrintService;
use App\Support\Gs1;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Http\Response;
use Illuminate\Validation\Rule;

class LabelTemplateController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        return LabelTemplateResource::collection(
            LabelTemplate::where('organization_id', $request->user()->organization_id)->orderByDesc('is_default')->orderBy('name')->get()
        );
    }

    public function store(Request $request): JsonResponse
    {
        $template = LabelTemplate::create($this->validated($request) + ['organization_id' => $request->user()->organization_id]);
        $this->syncDefault($template);

        return (new LabelTemplateResource($template))->response()->setStatusCode(201);
    }

    public function update(Request $request, int $id): LabelTemplateResource
    {
        $template = $this->find($request, $id);
        $template->update($this->validated($request));
        $this->syncDefault($template);

        return new LabelTemplateResource($template);
    }

    public function destroy(Request $request, int $id): Response
    {
        $this->find($request, $id)->delete();

        return response()->noContent();
    }

    public function preview(Request $request, int $id, PrintService $print): Response
    {
        $template = $this->find($request, $id);
        $org = $request->user()->organization;
        $product = Product::where('organization_id', $org->id)->find($request->query('product_id'))
            ?? Product::where('organization_id', $org->id)->orderBy('id')->first()
            ?? new Product(['gtin' => '04870712001017', 'name' => 'Футболка мужская базовая, белый, р. M', 'size' => 'M', 'composition' => 'хлопок 100%']);
        $code = new Code([
            'code' => '01'.$product->gtin.'21'.'SAMPLE0000001'.Gs1::GS.'91EE10'.Gs1::GS.'92'.str_repeat('dGVzdA', 7).'dGU=',
            'gtin' => $product->gtin,
        ]);
        $code->setRelation('product', $product);
        $pdf = $print->render($template, collect([$code]), 'pdf', $org->name.', '.$org->address);

        return response($pdf, 200, [
            'Content-Type' => 'application/pdf',
            'Content-Disposition' => 'inline; filename="obrazec.pdf"',
        ]);
    }

    private function validated(Request $request): array
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:100'],
            'size' => ['required', Rule::in(array_keys(LabelTemplate::SIZES))],
            'fields' => ['required', 'array'],
            'fields.*' => ['boolean'],
            'font_scale' => ['nullable', 'numeric', 'min:0.7', 'max:1.4'],
            'is_default' => ['nullable', 'boolean'],
        ]);
        $data['fields'] = array_intersect_key($data['fields'], array_flip(LabelTemplate::FIELDS));
        $data['font_scale'] ??= 1;
        $data['is_default'] = (bool) ($data['is_default'] ?? false);

        return $data;
    }

    private function syncDefault(LabelTemplate $template): void
    {
        if ($template->is_default) {
            LabelTemplate::where('organization_id', $template->organization_id)
                ->where('id', '!=', $template->id)
                ->update(['is_default' => false]);
        }
    }

    private function find(Request $request, int $id): LabelTemplate
    {
        return LabelTemplate::where('organization_id', $request->user()->organization_id)->findOrFail($id);
    }
}
