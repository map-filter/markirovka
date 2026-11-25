<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('organizations', function (Blueprint $table) {
            $table->id();
            $table->string('name');
            $table->string('bin', 12)->unique();
            $table->string('address')->nullable();
            $table->string('phone')->nullable();
            $table->string('integration_token_hash', 64)->nullable()->unique();
            $table->text('ismpt_token')->nullable();
            $table->timestamp('ismpt_token_expires_at')->nullable();
            $table->timestamps();
        });

        Schema::table('users', function (Blueprint $table) {
            $table->foreignId('organization_id')->nullable()->after('id')->constrained()->cascadeOnDelete();
            $table->string('role', 20)->default('operator')->after('email');
        });

        Schema::create('products', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->string('gtin', 14);
            $table->string('name', 500);
            $table->string('model', 100)->nullable();
            $table->string('size', 50)->nullable();
            $table->string('color', 100)->nullable();
            $table->string('composition', 255)->nullable();
            $table->string('tnved', 10)->nullable();
            $table->string('category', 100)->nullable();
            $table->string('brand', 100)->nullable();
            $table->string('country', 100)->nullable();
            $table->string('nkt_status', 20)->default('unchecked');
            $table->timestamp('nkt_checked_at')->nullable();
            $table->json('nkt_payload')->nullable();
            $table->timestamps();
            $table->unique(['organization_id', 'gtin']);
        });

        Schema::create('code_orders', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->foreignId('user_id')->nullable()->constrained()->nullOnDelete();
            $table->foreignId('product_id')->constrained()->cascadeOnDelete();
            $table->string('number', 20);
            $table->unsignedInteger('quantity');
            $table->string('status', 20)->default('new');
            $table->string('ismpt_order_id')->nullable();
            $table->text('error')->nullable();
            $table->unsignedInteger('attempts')->default(0);
            $table->timestamp('sent_at')->nullable();
            $table->timestamp('ready_at')->nullable();
            $table->timestamp('expires_at')->nullable();
            $table->timestamps();
            $table->unique(['organization_id', 'number']);
            $table->index(['organization_id', 'status']);
        });

        Schema::create('codes', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->foreignId('product_id')->constrained()->cascadeOnDelete();
            $table->foreignId('code_order_id')->constrained()->cascadeOnDelete();
            $table->text('code');
            $table->string('cis', 64);
            $table->string('gtin', 14);
            $table->string('serial', 32);
            $table->string('status', 20)->default('received');
            $table->timestamp('emitted_at')->nullable();
            $table->timestamp('expires_at')->nullable();
            $table->timestamp('printed_at')->nullable();
            $table->timestamp('applied_at')->nullable();
            $table->timestamp('introduced_at')->nullable();
            $table->timestamp('annulled_at')->nullable();
            $table->timestamps();
            $table->unique(['organization_id', 'cis']);
            $table->index(['organization_id', 'status']);
            $table->index(['organization_id', 'product_id', 'status']);
            $table->index('expires_at');
        });

        Schema::create('label_templates', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->string('name');
            $table->string('size', 10);
            $table->json('fields');
            $table->decimal('font_scale', 3, 2)->default(1);
            $table->boolean('is_default')->default(false);
            $table->timestamps();
        });

        Schema::create('print_agents', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->string('name');
            $table->string('token_hash', 64)->unique();
            $table->string('hostname')->nullable();
            $table->string('printer')->nullable();
            $table->string('version', 50)->nullable();
            $table->timestamp('last_seen_at')->nullable();
            $table->timestamps();
        });

        Schema::create('print_jobs', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->foreignId('user_id')->nullable()->constrained()->nullOnDelete();
            $table->foreignId('label_template_id')->nullable()->constrained()->nullOnDelete();
            $table->foreignId('print_agent_id')->nullable()->constrained()->nullOnDelete();
            $table->string('format', 10);
            $table->string('target', 10);
            $table->string('status', 20);
            $table->unsignedInteger('codes_count')->default(0);
            $table->string('file_path')->nullable();
            $table->text('error')->nullable();
            $table->timestamp('sent_at')->nullable();
            $table->timestamp('printed_at')->nullable();
            $table->timestamps();
            $table->index(['print_agent_id', 'status']);
        });

        Schema::create('code_print_job', function (Blueprint $table) {
            $table->foreignId('print_job_id')->constrained()->cascadeOnDelete();
            $table->foreignId('code_id')->constrained()->cascadeOnDelete();
            $table->primary(['print_job_id', 'code_id']);
        });

        Schema::create('documents', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->foreignId('user_id')->nullable()->constrained()->nullOnDelete();
            $table->string('type', 20);
            $table->string('status', 20)->default('draft');
            $table->longText('body');
            $table->string('ismpt_document_id')->nullable();
            $table->json('errors')->nullable();
            $table->unsignedInteger('codes_count')->default(0);
            $table->unsignedInteger('attempts')->default(0);
            $table->timestamp('sent_at')->nullable();
            $table->timestamp('processed_at')->nullable();
            $table->timestamps();
        });

        Schema::create('code_document', function (Blueprint $table) {
            $table->foreignId('document_id')->constrained()->cascadeOnDelete();
            $table->foreignId('code_id')->constrained()->cascadeOnDelete();
            $table->primary(['document_id', 'code_id']);
        });

        Schema::create('operations', function (Blueprint $table) {
            $table->id();
            $table->foreignId('organization_id')->constrained()->cascadeOnDelete();
            $table->foreignId('user_id')->nullable()->constrained()->nullOnDelete();
            $table->string('type', 50);
            $table->string('level', 10)->default('info');
            $table->text('message');
            $table->string('subject_type', 50)->nullable();
            $table->unsignedBigInteger('subject_id')->nullable();
            $table->json('payload')->nullable();
            $table->timestamp('created_at')->useCurrent();
            $table->index(['organization_id', 'created_at']);
        });
    }

    public function down(): void
    {
        foreach (['operations', 'code_document', 'documents', 'code_print_job', 'print_jobs', 'print_agents', 'label_templates', 'codes', 'code_orders', 'products'] as $name) {
            Schema::dropIfExists($name);
        }
        Schema::table('users', function (Blueprint $table) {
            $table->dropConstrainedForeignId('organization_id');
            $table->dropColumn('role');
        });
        Schema::dropIfExists('organizations');
    }
};
