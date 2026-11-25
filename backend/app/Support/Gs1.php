<?php

namespace App\Support;

use InvalidArgumentException;

class Gs1
{
    public const GS = "\x1D";

    public static function gtinCheckDigit(string $body): int
    {
        $sum = 0;
        $digits = str_split(strrev($body));
        foreach ($digits as $i => $digit) {
            $sum += (int) $digit * ($i % 2 === 0 ? 3 : 1);
        }

        return (10 - $sum % 10) % 10;
    }

    public static function isValidGtin(string $gtin): bool
    {
        if (! preg_match('/^\d{8}$|^\d{12,14}$/', $gtin)) {
            return false;
        }

        return self::gtinCheckDigit(substr($gtin, 0, -1)) === (int) substr($gtin, -1);
    }

    public static function normalizeGtin(string $gtin): string
    {
        $gtin = preg_replace('/\D/', '', $gtin) ?? '';

        return str_pad($gtin, 14, '0', STR_PAD_LEFT);
    }

    /**
     * @return array{gtin: string, serial: string, cis: string, key: ?string, crypto: ?string}
     */
    public static function parse(string $code): array
    {
        $code = ltrim($code, self::GS."\xE8");
        if (str_starts_with($code, '(')) {
            return self::parseHumanReadable($code);
        }
        if (! preg_match('/^01(\d{14})21([^\x1D]+)$/', explode(self::GS, $code, 2)[0], $m)) {
            throw new InvalidArgumentException('Код не похож на GS1: нет (01) GTIN и (21) серийного номера');
        }
        $parts = explode(self::GS, $code);
        $serial = $m[2];
        $key = null;
        $crypto = null;
        if (count($parts) === 1 && preg_match('/91.{4}92.{44}$/', $serial) && strlen($serial) > 13) {
            throw new InvalidArgumentException('Потерян символ GS: криптохвост слился с серийным номером');
        }
        foreach (array_slice($parts, 1) as $part) {
            if (str_starts_with($part, '91')) {
                $key = substr($part, 2);
            } elseif (str_starts_with($part, '92')) {
                $crypto = substr($part, 2);
            }
        }

        return [
            'gtin' => $m[1],
            'serial' => $serial,
            'cis' => '01'.$m[1].'21'.$serial,
            'key' => $key,
            'crypto' => $crypto,
        ];
    }

    /**
     * @return array{gtin: string, serial: string, cis: string, key: ?string, crypto: ?string}
     */
    private static function parseHumanReadable(string $code): array
    {
        if (! preg_match('/^\(01\)(\d{14})\(21\)(.+?)(?:\(91\)(.{4})(?:\(92\)(.+))?)?$/s', $code, $m)) {
            throw new InvalidArgumentException('Код не похож на GS1: нет (01) GTIN и (21) серийного номера');
        }

        return [
            'gtin' => $m[1],
            'serial' => $m[2],
            'cis' => '01'.$m[1].'21'.$m[2],
            'key' => ($m[3] ?? '') !== '' ? $m[3] : null,
            'crypto' => ($m[4] ?? '') !== '' ? $m[4] : null,
        ];
    }

    public static function humanReadable(string $code): string
    {
        $p = self::parse($code);
        $out = '(01)'.$p['gtin'].'(21)'.$p['serial'];
        if ($p['key'] !== null) {
            $out .= '(91)'.$p['key'];
        }
        if ($p['crypto'] !== null) {
            $out .= '(92)'.$p['crypto'];
        }

        return $out;
    }

    public static function fromHumanReadable(string $code): string
    {
        $p = self::parse($code);
        $out = '01'.$p['gtin'].'21'.$p['serial'];
        if ($p['key'] !== null) {
            $out .= self::GS.'91'.$p['key'];
        }
        if ($p['crypto'] !== null) {
            $out .= self::GS.'92'.$p['crypto'];
        }

        return $out;
    }

    public static function toZplField(string $code): string
    {
        return '~1'.str_replace(self::GS, '~1', $code);
    }
}
