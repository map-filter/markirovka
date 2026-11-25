<?php

use Illuminate\Support\Facades\Schedule;

Schedule::command('codes:maintain')->everyFiveMinutes()->withoutOverlapping();
Schedule::command('codes:maintain --remind')->dailyAt('03:00')->withoutOverlapping();
Schedule::command('ismpt:resume')->everyMinute()->withoutOverlapping();
