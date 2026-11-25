<?php

namespace App\Labels;

use App\Models\LabelTemplate;
use TCPDF;

class LabelRenderer
{
    private const DOTS_PER_MM = 8;

    /**
     * @param  iterable<LabelItem>  $items
     */
    public function pdf(LabelTemplate $template, iterable $items): string
    {
        $w = $template->widthMm();
        $h = $template->heightMm();
        $pdf = new TCPDF($w > $h ? 'L' : 'P', 'mm', [$w, $h], true, 'UTF-8', false);
        $pdf->SetCreator('Маркировка за день');
        $pdf->SetTitle('Этикетки');
        $pdf->setPrintHeader(false);
        $pdf->setPrintFooter(false);
        $pdf->SetMargins(0, 0, 0);
        $pdf->SetAutoPageBreak(false, 0);
        $pdf->setCellPaddings(0, 0, 0, 0);
        $pdf->setCellHeightRatio(1.1);

        foreach ($items as $item) {
            $pdf->AddPage();
            $this->drawPdfLabel($pdf, $template, $item);
        }

        return $pdf->Output('labels.pdf', 'S');
    }

    private function drawPdfLabel(TCPDF $pdf, LabelTemplate $template, LabelItem $item): void
    {
        $small = $template->size === '43x25';
        $scale = $template->font_scale ?: 1;
        $margin = $small ? 1.5 : 2.0;
        $dm = $small ? 17.0 : 22.0;
        $w = $template->widthMm();
        $h = $template->heightMm();

        $textX = $margin;
        if ($template->shows('datamatrix')) {
            $this->drawMatrix($pdf, $item->code, $margin, $margin, $dm);
            $textX = $margin + $dm + ($small ? 1.5 : 2.0);
        }
        $textW = $w - $textX - $margin;

        if ($template->shows('serial') && $template->shows('datamatrix')) {
            $pdf->SetFont('dejavusans', '', ($small ? 3.6 : 4.4) * $scale);
            $pdf->MultiCell($dm + 1, 4, $item->serialLine(), 0, 'L', false, 1, $margin, $margin + $dm + 0.6, true, 0, false, true, $small ? 3 : 4, 'T', true);
        }

        $y = $margin;
        $bottom = $template->shows('manufacturer') ? $h - ($small ? 4.5 : 6.5) : $h - $margin;
        if ($template->shows('name')) {
            $maxH = $small ? 7.0 : 11.0;
            $fontSize = ($small ? 6 : 7.5) * $scale;
            $pdf->SetFont('dejavusans', 'B', $fontSize);
            while ($fontSize > 4.5 && $pdf->getStringHeight($textW, $item->name) > $maxH) {
                $fontSize -= 0.25;
                $pdf->SetFont('dejavusans', 'B', $fontSize);
            }
            $y = $this->textBlock($pdf, $item->name, $textX, $y, $textW, $maxH) + 0.8;
        }
        $lines = [];
        if ($template->shows('size') && $item->size) {
            $lines[] = ['Размер: '.$item->size, 'B'];
        }
        if ($template->shows('composition') && $item->composition) {
            $lines[] = ['Состав: '.$item->composition, ''];
        }
        if ($template->shows('gtin')) {
            $lines[] = ['GTIN '.$item->gtin, ''];
        }
        foreach ($lines as [$text, $style]) {
            $pdf->SetFont('dejavusans', $style, ($small ? 5 : 6) * $scale);
            if ($y + $pdf->getStringHeight($textW, $text) > $bottom + 0.2) {
                continue;
            }
            $y = $this->textBlock($pdf, $text, $textX, $y, $textW, $bottom - $y) + 0.4;
        }

        if ($template->shows('manufacturer')) {
            $pdf->SetFont('dejavusans', '', ($small ? 3.8 : 4.6) * $scale);
            $mh = $small ? 3.6 : 5.2;
            $mx = $small && $template->shows('datamatrix') ? $textX : $margin;
            $pdf->MultiCell($w - $mx - $margin, $mh, 'Изготовитель: '.$item->manufacturer, 0, 'L', false, 1, $mx, $h - $margin - $mh + ($small ? 0.6 : 0.4), true, 0, false, true, $mh, 'B', true);
        }
    }

    private function textBlock(TCPDF $pdf, string $text, float $x, float $y, float $w, float $maxH): float
    {
        $height = min($pdf->getStringHeight($w, $text), $maxH);
        $pdf->MultiCell($w, $height, $text, 0, 'L', false, 1, $x, $y, true, 0, false, true, $maxH, 'T', false);

        return $y + $height;
    }

    private function drawMatrix(TCPDF $pdf, string $code, float $x, float $y, float $size): void
    {
        $matrix = DataMatrix::matrix($code);
        $module = $size / count($matrix);
        $pdf->SetFillColor(0, 0, 0);
        foreach ($matrix as $row => $cells) {
            $run = null;
            foreach ($cells as $col => $on) {
                if ($on && $run === null) {
                    $run = $col;
                }
                if ((! $on || $col === count($cells) - 1) && $run !== null) {
                    $end = $on ? $col + 1 : $col;
                    $pdf->Rect($x + $run * $module, $y + $row * $module, ($end - $run) * $module, $module, 'F');
                    $run = null;
                }
            }
        }
    }

    /**
     * @param  iterable<LabelItem>  $items
     */
    public function zpl(LabelTemplate $template, iterable $items): string
    {
        $out = '';
        foreach ($items as $item) {
            $out .= $this->zplLabel($template, $item);
        }

        return $out;
    }

    private function zplLabel(LabelTemplate $template, LabelItem $item): string
    {
        $small = $template->size === '43x25';
        $scale = $template->font_scale ?: 1;
        $d = self::DOTS_PER_MM;
        $w = $template->widthMm() * $d;
        $h = $template->heightMm() * $d;
        $margin = (int) round(($small ? 1.5 : 2) * $d);
        $module = $small ? 4 : 5;
        $dmDots = 36 * $module;

        $z = ['^XA', '^CI28', "^PW{$w}", "^LL{$h}", '^LH0,0'];
        $textX = $margin;
        if ($template->shows('datamatrix')) {
            $z[] = "^FO{$margin},{$margin}^BXN,{$module},200,0,0,1,~,1^FD".\App\Support\Gs1::toZplField($item->code).'^FS';
            $textX = $margin + $dmDots + $margin;
        }
        $textW = $w - $textX - $margin;
        $font = fn (float $mm) => (int) round($mm * $d * $scale);

        if ($template->shows('serial') && $template->shows('datamatrix')) {
            $z[] = sprintf('^FO%d,%d^A0N,%d,%d^FB%d,2,0,L^FH_^FD%s^FS', $margin, $margin + $dmDots + 4, $font(1.5), $font(1.3), $dmDots + 8, $this->zplText($item->serialLine()));
        }

        $y = $margin;
        if ($template->shows('name')) {
            $lines = $small ? 2 : 3;
            $size = $font($small ? 2.2 : 2.7);
            $z[] = sprintf('^FO%d,%d^A0N,%d,%d^FB%d,%d,2,L^FH_^FD%s^FS', $textX, $y, $size, $size, $textW, $lines, $this->zplText($item->name));
            $y += ($size + 2) * $lines + 6;
        }
        $rows = [];
        if ($template->shows('size') && $item->size) {
            $rows[] = 'Размер: '.$item->size;
        }
        if ($template->shows('composition') && $item->composition && ! $small) {
            $rows[] = 'Состав: '.$item->composition;
        }
        if ($template->shows('gtin')) {
            $rows[] = 'GTIN '.$item->gtin;
        }
        $size = $font($small ? 1.9 : 2.1);
        foreach ($rows as $row) {
            $z[] = sprintf('^FO%d,%d^A0N,%d,%d^FB%d,2,0,L^FH_^FD%s^FS', $textX, $y, $size, $size, $textW, $this->zplText($row));
            $y += ($size + 2) * (mb_strlen($row) * $size * 0.5 > $textW ? 2 : 1) + 4;
        }

        if ($template->shows('manufacturer')) {
            $size = $font($small ? 1.5 : 1.7);
            $mx = $small && $template->shows('datamatrix') ? $textX : $margin;
            $z[] = sprintf('^FO%d,%d^A0N,%d,%d^FB%d,2,0,L^FH_^FD%s^FS', $mx, $h - $margin - ($size + 2) * 2, $size, $size, $w - $mx - $margin, $this->zplText('Изготовитель: '.$item->manufacturer));
        }

        $z[] = '^PQ1';
        $z[] = '^XZ';

        return implode("\n", $z)."\n";
    }

    private function zplText(string $text): string
    {
        return strtr($text, ['_' => '_5F', '^' => '_5E', '~' => '_7E']);
    }
}
