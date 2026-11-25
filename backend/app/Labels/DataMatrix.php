<?php

namespace App\Labels;

use Com\Tecnick\Barcode\Barcode;

class DataMatrix
{
    /**
     * @return list<list<bool>>
     */
    public static function matrix(string $code): array
    {
        $grid = (new Barcode)->getBarcodeObj('DATAMATRIX,S,GS1', "\xE8".ltrim($code, "\x1D\xE8"), -1, -1, 'black')->getGrid('0', '1');
        $rows = [];
        foreach (explode("\n", trim($grid)) as $line) {
            $rows[] = array_map(fn (string $c) => $c === '1', str_split(trim($line)));
        }

        return $rows;
    }

    public static function svg(string $code, int $module = 4, int $quiet = 2): string
    {
        $matrix = self::matrix($code);
        $size = (count($matrix) + $quiet * 2) * $module;
        $rects = '';
        foreach ($matrix as $y => $row) {
            foreach ($row as $x => $on) {
                if ($on) {
                    $rects .= sprintf('<rect x="%d" y="%d" width="%d" height="%d"/>', ($x + $quiet) * $module, ($y + $quiet) * $module, $module, $module);
                }
            }
        }

        return sprintf('<svg xmlns="http://www.w3.org/2000/svg" width="%1$d" height="%1$d" viewBox="0 0 %1$d %1$d"><rect width="100%%" height="100%%" fill="#fff"/><g fill="#000">%2$s</g></svg>', $size, $rects);
    }
}
