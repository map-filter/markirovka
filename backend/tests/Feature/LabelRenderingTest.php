<?php

namespace Tests\Feature;

use App\Labels\DataMatrix;
use App\Labels\LabelItem;
use App\Labels\LabelRenderer;
use App\Models\LabelTemplate;
use Tests\TestCase;

class LabelRenderingTest extends TestCase
{
    private function item(): LabelItem
    {
        return new LabelItem($this->sampleCode(serial: 'aB3%&(x9Q:<=K'), 'Худи унисекс, серый, р. XL', 'XL', 'хлопок 80%, полиэстер 20%', '04870712001017', 'ТОО «Кендір Тігін», г. Павлодар');
    }

    public function test_data_matrix_is_square_ecc200(): void
    {
        $matrix = DataMatrix::matrix($this->sampleCode());

        $this->assertCount(count($matrix[0]), $matrix);
        $this->assertGreaterThanOrEqual(32, count($matrix));
        $this->assertTrue($matrix[count($matrix) - 1][0]);
    }

    public function test_pdf_has_one_page_per_label_for_both_sizes(): void
    {
        foreach (['58x40', '43x25'] as $size) {
            $template = new LabelTemplate(['name' => 'x', 'size' => $size, 'fields' => array_fill_keys(LabelTemplate::FIELDS, true), 'font_scale' => 1]);
            $pdf = (new LabelRenderer)->pdf($template, [$this->item(), $this->item(), $this->item()]);

            $this->assertStringStartsWith('%PDF-', $pdf);
            $this->assertSame(3, preg_match_all('#/Type /Page\b#', $pdf));
        }
    }

    public function test_zpl_uses_gs1_data_matrix_with_fnc1(): void
    {
        $template = new LabelTemplate(['name' => 'x', 'size' => '58x40', 'fields' => array_fill_keys(LabelTemplate::FIELDS, true), 'font_scale' => 1]);
        $zpl = (new LabelRenderer)->zpl($template, [$this->item(), $this->item()]);

        $this->assertSame(2, substr_count($zpl, '^XA'));
        $this->assertSame(2, substr_count($zpl, '^XZ'));
        $this->assertStringContainsString('^BXN,5,200,0,0,1,~,1^FD~1010487071200101721aB3%&(x9Q:<=K~191EE10~192', $zpl);
        $this->assertStringContainsString('^CI28', $zpl);
        $this->assertStringContainsString('^PW464', $zpl);
    }
}
