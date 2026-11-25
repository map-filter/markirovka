<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\PrintJobResource;
use App\Models\PrintJob;
use App\Services\PrintService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Support\Facades\Storage;
use Symfony\Component\HttpFoundation\Response;

class PrintJobController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        return PrintJobResource::collection(
            PrintJob::with(['template', 'agent'])
                ->where('organization_id', $request->user()->organization_id)
                ->latest('id')
                ->paginate(20)
        );
    }

    public function store(Request $request, PrintService $print): JsonResponse
    {
        $data = $request->validate([
            'template_id' => ['required', 'integer'],
            'format' => ['required', 'in:pdf,zpl'],
            'target' => ['required', 'in:download,agent'],
            'agent_id' => ['nullable', 'integer'],
            'code_ids' => ['nullable', 'array', 'max:5000'],
            'code_ids.*' => ['integer'],
            'product_id' => ['nullable', 'integer'],
            'order_id' => ['nullable', 'integer'],
            'quantity' => ['nullable', 'integer', 'min:1', 'max:5000'],
            'reprint' => ['nullable', 'boolean'],
        ]);
        $job = $print->create($request->user(), $data);

        return (new PrintJobResource($job->load(['template', 'agent'])))->response()->setStatusCode(201);
    }

    public function show(Request $request, int $id): PrintJobResource
    {
        return new PrintJobResource($this->find($request, $id)->load(['template', 'agent']));
    }

    public function download(Request $request, int $id): Response
    {
        $job = $this->find($request, $id);
        abort_unless($job->file_path && Storage::disk('local')->exists($job->file_path), 404);
        $type = $job->format === 'pdf' ? 'application/pdf' : 'text/plain; charset=UTF-8';

        return Storage::disk('local')->download($job->file_path, "labels-{$job->id}.{$job->format}", ['Content-Type' => $type]);
    }

    private function find(Request $request, int $id): PrintJob
    {
        return PrintJob::where('organization_id', $request->user()->organization_id)->findOrFail($id);
    }
}
