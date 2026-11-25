<?php

namespace App\Console\Commands;

use App\Models\Organization;
use App\Services\CodeLifecycle;
use Illuminate\Console\Command;

class CodesMaintain extends Command
{
    protected $signature = 'codes:maintain {--remind : Записать напоминания о кодах, которые скоро аннулируются}';

    protected $description = 'Аннулирует просроченные коды, сверяет статусы с ИС МПТ, пишет напоминания';

    public function handle(CodeLifecycle $lifecycle): int
    {
        foreach (Organization::all() as $org) {
            $synced = $lifecycle->syncWithIsmpt($org);
            $annulled = $lifecycle->annulExpired($org);
            $reminded = $this->option('remind') ? $lifecycle->remindExpiring($org) : 0;
            $this->line("{$org->bin}: ИС МПТ {$synced}, срок {$annulled}, напоминаний {$reminded}");
        }

        return self::SUCCESS;
    }
}
