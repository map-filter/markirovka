<?php

namespace App\Ismpt;

class EmulatedSignature
{
    public static function sign(string $data, string $bin): string
    {
        return base64_encode(implode("\n", [
            'MKEMU-CMS-v1',
            base64_encode($data),
            'BIN:'.$bin,
            'SIG:'.hash('sha256', $data.$bin),
        ]));
    }
}
