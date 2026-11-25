<?php

namespace App\Services;

use App\Models\Organization;
use App\Models\Product;
use App\Support\Gs1;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Validator;
use OpenSpout\Reader\XLSX\Reader as XlsxReader;

class ProductImporter
{
    public const HEADERS = [
        'gtin' => 'GTIN',
        'name' => 'Наименование',
        'model' => 'Модель',
        'size' => 'Размер',
        'color' => 'Цвет',
        'composition' => 'Состав',
        'tnved' => 'ТН ВЭД',
        'category' => 'Категория',
        'brand' => 'Бренд',
        'country' => 'Страна',
    ];

    private const ALIASES = [
        'gtin' => ['gtin', 'штрихкод', 'штрих-код', 'ean'],
        'name' => ['наименование', 'название', 'товар', 'name'],
        'model' => ['модель', 'артикул', 'model'],
        'size' => ['размер', 'size'],
        'color' => ['цвет', 'color'],
        'composition' => ['состав', 'состав ткани', 'composition'],
        'tnved' => ['тн вэд', 'тнвэд', 'код тн вэд', 'tnved'],
        'category' => ['категория', 'группа', 'category'],
        'brand' => ['бренд', 'торговая марка', 'brand'],
        'country' => ['страна', 'страна производства', 'country'],
    ];

    /**
     * @return array{created: int, updated: int, errors: list<array{row: int, message: string}>}
     */
    public function import(Organization $organization, UploadedFile $file): array
    {
        $rows = strtolower($file->getClientOriginalExtension()) === 'xlsx'
            ? $this->readXlsx($file->getRealPath())
            : $this->readCsv($file->getRealPath());

        $result = ['created' => 0, 'updated' => 0, 'errors' => []];
        if ($rows === []) {
            $result['errors'][] = ['row' => 1, 'message' => 'Файл пустой'];

            return $result;
        }

        $map = $this->mapHeader(array_shift($rows));
        if (! isset($map['gtin'], $map['name'])) {
            $result['errors'][] = ['row' => 1, 'message' => 'В первой строке нужны колонки «GTIN» и «Наименование»'];

            return $result;
        }

        foreach ($rows as $index => $row) {
            $line = $index + 2;
            $data = [];
            foreach ($map as $field => $col) {
                $value = trim((string) ($row[$col] ?? ''));
                $data[$field] = $value === '' ? null : $value;
            }
            if (array_filter($data) === []) {
                continue;
            }
            $data['gtin'] = Gs1::normalizeGtin((string) $data['gtin']);
            if (isset($data['tnved'])) {
                $data['tnved'] = preg_replace('/\D/', '', $data['tnved']);
            }
            $validator = Validator::make($data, [
                'gtin' => ['required', new \App\Rules\Gtin],
                'name' => ['required', 'max:500'],
                'tnved' => ['nullable', 'regex:/^\d{10}$/'],
            ], ['tnved.regex' => 'Код ТН ВЭД: 10 цифр', 'name.required' => 'Не заполнено наименование']);
            if ($validator->fails()) {
                $result['errors'][] = ['row' => $line, 'message' => implode('; ', $validator->errors()->all())];

                continue;
            }
            $product = Product::firstOrNew(['organization_id' => $organization->id, 'gtin' => $data['gtin']]);
            $exists = $product->exists;
            $product->fill(array_filter($data, fn ($v) => $v !== null));
            $product->save();
            $exists ? $result['updated']++ : $result['created']++;
        }

        return $result;
    }

    /**
     * @param  list<string|null>  $header
     * @return array<string, int>
     */
    private function mapHeader(array $header): array
    {
        $map = [];
        foreach ($header as $i => $title) {
            $title = mb_strtolower(trim(str_replace("\xEF\xBB\xBF", '', (string) $title)));
            foreach (self::ALIASES as $field => $aliases) {
                if (! isset($map[$field]) && in_array($title, $aliases, true)) {
                    $map[$field] = $i;
                }
            }
        }

        return $map;
    }

    /**
     * @return list<list<string|null>>
     */
    private function readCsv(string $path): array
    {
        $content = (string) file_get_contents($path);
        if (! mb_check_encoding($content, 'UTF-8')) {
            $content = mb_convert_encoding($content, 'UTF-8', 'Windows-1251');
        }
        $content = preg_replace('/^\xEF\xBB\xBF/', '', $content);
        $firstLine = strtok($content, "\n") ?: '';
        $delimiter = substr_count($firstLine, ';') >= substr_count($firstLine, ',') ? ';' : ',';
        $stream = fopen('php://temp', 'r+');
        fwrite($stream, $content);
        rewind($stream);
        $rows = [];
        while (($row = fgetcsv($stream, 0, $delimiter, '"', '')) !== false) {
            $rows[] = $row;
        }
        fclose($stream);

        return $rows;
    }

    /**
     * @return list<list<string|null>>
     */
    private function readXlsx(string $path): array
    {
        $reader = new XlsxReader;
        $reader->open($path);
        $rows = [];
        foreach ($reader->getSheetIterator() as $sheet) {
            foreach ($sheet->getRowIterator() as $row) {
                $rows[] = array_map(function ($cell) {
                    $value = $cell->getValue();
                    if (is_float($value) && floor($value) === $value) {
                        return number_format($value, 0, '', '');
                    }

                    return $value instanceof \DateTimeInterface ? $value->format('Y-m-d') : (string) $value;
                }, $row->getCells());
            }
            break;
        }
        $reader->close();

        return $rows;
    }
}
