<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Ismpt\EmulatedSignature;
use App\Ismpt\IsmptSession;
use App\Jobs\ProcessCodeOrder;
use App\Jobs\ProcessDocument;
use App\Models\CodeOrder;
use App\Models\Document;
use App\Services\NktService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class IsmptController extends Controller
{
    public function session(Request $request, IsmptSession $ismpt): JsonResponse
    {
        return response()->json($ismpt->status($request->user()->organization));
    }

    public function key(IsmptSession $ismpt): JsonResponse
    {
        return response()->json($ismpt->client()->authKey());
    }

    public function signIn(Request $request, IsmptSession $ismpt, NktService $nkt): JsonResponse
    {
        $data = $request->validate([
            'uuid' => ['required', 'string', 'max:100'],
            'signature' => ['required', 'string'],
        ]);
        $org = $request->user()->organization;
        $ismpt->connect($org, $data['uuid'], $data['signature']);

        rescue(fn () => $nkt->syncAll($org->fresh()), report: false);
        CodeOrder::where('organization_id', $org->id)->whereIn('status', CodeOrder::ACTIVE)->pluck('id')
            ->each(fn (int $id) => ProcessCodeOrder::dispatch($id));
        Document::where('organization_id', $org->id)->whereIn('status', [Document::SENT, Document::PROCESSING])->pluck('id')
            ->each(fn (int $id) => ProcessDocument::dispatch($id));

        return response()->json($ismpt->status($org->fresh()));
    }

    public function emulateSign(Request $request): JsonResponse
    {
        abort_unless(config('ismpt.sign_mode') === 'emulate', 403, 'Эмуляция подписи выключена');
        $data = $request->validate(['data_base64' => ['required', 'string']]);
        $raw = base64_decode($data['data_base64'], true);
        abort_if($raw === false, 422, 'data_base64: неверный base64');

        return response()->json(['signature' => EmulatedSignature::sign($raw, $request->user()->organization->bin)]);
    }
}
