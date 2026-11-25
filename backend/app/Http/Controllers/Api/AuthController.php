<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\OrganizationResource;
use App\Http\Resources\UserResource;
use App\Models\Operation;
use App\Models\User;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Facades\Hash;
use Illuminate\Validation\ValidationException;

class AuthController extends Controller
{
    public function login(Request $request): JsonResponse
    {
        $data = $request->validate([
            'email' => ['required', 'email'],
            'password' => ['required', 'string'],
        ]);

        $user = User::whereRaw('lower(email) = ?', [mb_strtolower($data['email'])])->first();
        if (! $user || ! Hash::check($data['password'], $user->password)) {
            throw ValidationException::withMessages(['email' => 'Неверный email или пароль']);
        }

        $token = $user->createToken('cabinet', ['*'], now()->addDays(14))->plainTextToken;
        Operation::log($user->organization_id, 'auth.login', "Вход в кабинет: {$user->name}", userId: $user->id);

        return response()->json([
            'token' => $token,
            'user' => new UserResource($user),
        ]);
    }

    public function logout(Request $request): Response
    {
        $request->user()->currentAccessToken()?->delete();

        return response()->noContent();
    }

    public function me(Request $request): JsonResponse
    {
        $user = $request->user();

        return response()->json([
            'user' => new UserResource($user),
            'organization' => new OrganizationResource($user->organization),
        ]);
    }

    public function config(): JsonResponse
    {
        return response()->json([
            'sign_mode' => config('ismpt.sign_mode'),
            'ismpt_mode' => config('ismpt.mode'),
            'ismpt_base_url' => config('ismpt.base_url'),
            'emulator_console_url' => config('ismpt.mode') === 'emulator' ? config('ismpt.emulator_console_url') : null,
            'annul_days' => (int) config('ismpt.annul_days'),
        ]);
    }
}
