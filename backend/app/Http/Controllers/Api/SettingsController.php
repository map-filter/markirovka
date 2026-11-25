<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\OrganizationResource;
use App\Http\Resources\PrintAgentResource;
use App\Models\Operation;
use App\Models\PrintAgent;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Str;

class SettingsController extends Controller
{
    public function show(Request $request): JsonResponse
    {
        $org = $request->user()->organization;

        return response()->json([
            'organization' => new OrganizationResource($org),
            'integration_token_set' => $org->integration_token_hash !== null,
            'agents' => PrintAgentResource::collection($org->printAgents()->orderBy('name')->get()),
        ]);
    }

    public function update(Request $request): JsonResponse
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:255'],
            'address' => ['nullable', 'string', 'max:255'],
            'phone' => ['nullable', 'string', 'max:50'],
        ]);
        $org = $request->user()->organization;
        $org->update($data);
        Operation::log($org->id, 'settings.updated', 'Изменены реквизиты организации');

        return response()->json(['organization' => new OrganizationResource($org)]);
    }

    public function integrationToken(Request $request): JsonResponse
    {
        $org = $request->user()->organization;
        $token = 'mk1c_'.Str::random(40);
        $org->forceFill(['integration_token_hash' => hash('sha256', $token)])->save();
        Operation::log($org->id, 'settings.integration_token', 'Выпущен новый токен для 1С', 'warning');

        return response()->json(['token' => $token]);
    }

    public function storeAgent(Request $request): JsonResponse
    {
        $data = $request->validate(['name' => ['required', 'string', 'max:100']]);
        $token = 'mkag_'.Str::random(40);
        $agent = PrintAgent::create([
            'organization_id' => $request->user()->organization_id,
            'name' => $data['name'],
            'token_hash' => PrintAgent::hashToken($token),
        ]);
        Operation::log($agent->organization_id, 'agent.created', "Подключён агент печати «{$agent->name}»", subject: $agent);

        return response()->json(['data' => new PrintAgentResource($agent), 'token' => $token], 201);
    }

    public function destroyAgent(Request $request, int $id): Response
    {
        $agent = PrintAgent::where('organization_id', $request->user()->organization_id)->findOrFail($id);
        $agent->delete();
        Operation::log($agent->organization_id, 'agent.deleted', "Отключён агент печати «{$agent->name}»", 'warning');

        return response()->noContent();
    }
}
