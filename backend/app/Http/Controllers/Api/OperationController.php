<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\OperationResource;
use App\Models\Operation;
use Illuminate\Http\Request;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;

class OperationController extends Controller
{
    public function index(Request $request): AnonymousResourceCollection
    {
        $query = Operation::with('user')
            ->where('organization_id', $request->user()->organization_id)
            ->orderByDesc('id');
        if ($type = $request->query('type')) {
            $query->where('type', 'like', str_replace(['%', '_'], ['\%', '\_'], $type).'%');
        }
        if ($level = $request->query('level')) {
            $query->where('level', $level);
        }

        return OperationResource::collection($query->paginate(50));
    }
}
