<?php

namespace Tests\Unit;

use App\Support\Gs1;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class Gs1Test extends TestCase
{
    private const CODE = "010487071200101721aB3%&(x9Q:<=K\x1D91EE10\x1D92Ab0+Ab0+Ab0+Ab0+Ab0+Ab0+Ab0+Ab0+Ab0+Ab0+Ab0+";

    public function test_gtin_check_digit(): void
    {
        $this->assertTrue(Gs1::isValidGtin('04870712001017'));
        $this->assertFalse(Gs1::isValidGtin('04870712001018'));
        $this->assertSame('04870712001017', Gs1::normalizeGtin('4870712001017'));
    }

    public function test_parses_full_code_with_group_separators(): void
    {
        $parsed = Gs1::parse(self::CODE);

        $this->assertSame('04870712001017', $parsed['gtin']);
        $this->assertSame('aB3%&(x9Q:<=K', $parsed['serial']);
        $this->assertSame('010487071200101721aB3%&(x9Q:<=K', $parsed['cis']);
        $this->assertSame('EE10', $parsed['key']);
        $this->assertSame(44, strlen((string) $parsed['crypto']));
    }

    public function test_detects_lost_group_separator(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Потерян символ GS');

        Gs1::parse(str_replace("\x1D", '', self::CODE));
    }

    public function test_human_readable_round_trip(): void
    {
        $hr = Gs1::humanReadable(self::CODE);

        $this->assertStringStartsWith('(01)04870712001017(21)aB3%&(x9Q:<=K(91)EE10(92)', $hr);
        $this->assertSame(self::CODE, Gs1::fromHumanReadable($hr));
    }

    public function test_zpl_field_keeps_fnc1_at_start_and_between_fields(): void
    {
        $field = Gs1::toZplField(self::CODE);

        $this->assertStringStartsWith('~1010487071200101721', $field);
        $this->assertSame(3, substr_count($field, '~1'));
        $this->assertStringNotContainsString("\x1D", $field);
    }
}
