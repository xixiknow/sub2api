<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="card p-5">
            <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="dollar" size="sm" class="text-primary-500" />
              {{ t('affiliate.stats.rebateRate') }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formattedRebateRate }}<span class="ml-0.5 text-base font-medium">%</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('affiliate.stats.rebateRateHint') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCount(detail.aff_count) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <div
            v-for="item in levelRebateSummaries"
            :key="item.level"
            class="card p-5"
          >
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('affiliate.stats.levelRebate', { level: item.level }) }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(item.rebate_amount) }}
            </p>
          </div>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.description') }}</p>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
            <p class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('affiliate.tips.title') }}</p>
            <ul class="mt-2 space-y-1 text-sm text-primary-700 dark:text-primary-300">
              <li>1. {{ t('affiliate.tips.line1') }}</li>
              <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>3. {{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
            </ul>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-terra-line px-6 py-5 dark:border-dark-700">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <p class="inline-flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-primary-700 dark:text-primary-300">
                  <Icon name="lightbulb" size="xs" />
                  {{ t('affiliate.rebateGuide.eyebrow') }}
                </p>
                <h3 class="mt-2 text-base font-semibold text-terra-ink dark:text-white">
                  {{ t('affiliate.rebateGuide.title') }}
                </h3>
                <p class="mt-1 max-w-2xl text-sm leading-6 text-terra-muted dark:text-dark-400">
                  {{ t('affiliate.rebateGuide.subtitle') }}
                </p>
              </div>
              <div class="grid w-full max-w-sm grid-cols-2 gap-2 rounded-lg border border-terra-line bg-terra-surface p-2 dark:border-dark-700 dark:bg-dark-900/70">
                <div class="rounded-md px-3 py-2">
                  <p class="text-xs font-medium text-terra-muted dark:text-dark-400">
                    {{ t('affiliate.rebateGuide.invitedLabel') }}
                  </p>
                  <p class="mt-1 text-lg font-semibold text-terra-ink dark:text-white">
                    {{ formatCount(qualifiedInviteCount) }}
                  </p>
                  <p class="mt-0.5 text-xs text-terra-muted dark:text-dark-400">
                    {{ t('affiliate.rebateGuide.totalInvites', { count: formatCount(detail.aff_count) }) }}
                  </p>
                </div>
                <div class="rounded-md bg-white px-3 py-2 shadow-glass-sm dark:bg-dark-800">
                  <p class="text-xs font-medium text-terra-muted dark:text-dark-400">
                    {{ t('affiliate.rebateGuide.finalGoalLabel') }}
                  </p>
                  <p class="mt-1 text-lg font-semibold text-primary-700 dark:text-primary-300">
                    {{ formatCount(level3UnlockThreshold) }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <div class="grid lg:grid-cols-[0.92fr_1.08fr]">
            <div class="border-b border-terra-line p-6 dark:border-dark-700 lg:border-b-0 lg:border-r">
              <div class="flex items-center justify-between gap-3">
                <h4 class="text-sm font-semibold text-terra-ink dark:text-white">
                  {{ t('affiliate.rebateGuide.mechanicsTitle') }}
                </h4>
                <span class="text-xs font-medium text-terra-muted dark:text-dark-400">
                  {{ t('affiliate.rebateGuide.autoSettle') }}
                </span>
              </div>

              <div class="mt-4 space-y-4">
                <div
                  v-for="item in rebateMechanismItems"
                  :key="item.level"
                  class="flex gap-3"
                >
                  <div
                    class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-sm font-bold"
                    :class="item.unlocked
                      ? 'bg-primary-50 text-primary-700 ring-1 ring-primary-100 dark:bg-primary-900/25 dark:text-primary-200 dark:ring-primary-800/50'
                      : 'bg-gray-100 text-gray-500 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-dark-400 dark:ring-dark-700'"
                  >
                    {{ item.level }}
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <p class="font-medium text-terra-ink dark:text-white">{{ item.title }}</p>
                      <span
                        class="inline-flex rounded-md px-2 py-0.5 text-[11px] font-semibold"
                        :class="item.unlocked
                          ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-200 dark:ring-emerald-800/50'
                          : 'bg-gray-100 text-gray-600 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700'"
                      >
                        {{ item.statusLabel }}
                      </span>
                    </div>
                    <p class="mt-1 text-sm leading-6 text-terra-muted dark:text-dark-400">
                      {{ item.description }}
                    </p>
                    <p class="mt-1 text-xs font-semibold text-primary-700 dark:text-primary-300">
                      {{ item.rateLabel }}
                    </p>
                  </div>
                </div>
              </div>

              <div class="mt-6 rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70">
                <p class="text-sm font-semibold text-terra-ink dark:text-white">
                  {{ t('affiliate.rebateGuide.flowTitle') }}
                </p>
                <div class="mt-3 grid gap-2 sm:grid-cols-4">
                  <div
                    v-for="step in rebateFlowItems"
                    :key="step"
                    class="min-w-0 rounded-md bg-white px-3 py-2 text-xs font-medium text-terra-muted shadow-glass-sm dark:bg-dark-800 dark:text-dark-300"
                  >
                    {{ step }}
                  </div>
                </div>
              </div>
            </div>

            <div class="p-6">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h4 class="text-sm font-semibold text-terra-ink dark:text-white">
                    {{ t('affiliate.rebateGuide.progressTitle') }}
                  </h4>
                  <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">
                    {{ t('affiliate.rebateGuide.progressSubtitle') }}
                  </p>
                </div>
                <div class="rounded-lg bg-primary-50 px-3 py-2 text-sm font-semibold text-primary-800 ring-1 ring-primary-100 dark:bg-primary-900/25 dark:text-primary-200 dark:ring-primary-800/50">
                  <span v-if="nextLockedProgress">
                    {{ t('affiliate.rebateGuide.nextGoal', {
                      level: nextLockedProgress.level,
                      count: formatCount(nextLockedProgress.remaining),
                    }) }}
                  </span>
                  <span v-else>{{ t('affiliate.rebateGuide.allUnlocked') }}</span>
                </div>
              </div>

              <div class="mt-5 space-y-4">
                <div
                  v-for="row in unlockProgressRows"
                  :key="row.level"
                  class="rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70"
                >
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div class="min-w-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <p class="font-semibold text-terra-ink dark:text-white">{{ row.title }}</p>
                        <span
                          class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-semibold"
                          :class="row.unlocked
                            ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-200 dark:ring-emerald-800/50'
                            : 'bg-amber-50 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-900/25 dark:text-amber-200 dark:ring-amber-800/50'"
                        >
                          <Icon :name="row.unlocked ? 'checkCircle' : 'lock'" size="xs" />
                          {{ row.statusLabel }}
                        </span>
                      </div>
                      <p class="mt-1 text-sm leading-6 text-terra-muted dark:text-dark-400">
                        {{ row.requirementLabel }}
                      </p>
                    </div>
                    <div class="shrink-0 text-left sm:text-right">
                      <p class="text-lg font-semibold text-primary-700 dark:text-primary-300">
                        {{ row.rateLabel }}
                      </p>
                      <p class="text-xs text-terra-muted dark:text-dark-400">
                        {{ levelRateSourceLabel(row.source) }}
                      </p>
                    </div>
                  </div>
                  <div class="mt-4">
                    <div class="flex items-center justify-between text-xs text-terra-muted dark:text-dark-400">
                      <span>{{ row.progressLabel }}</span>
                      <span>{{ row.progressText }}</span>
                    </div>
                    <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-800">
                      <div
                        class="h-full rounded-full transition-all"
                        :class="row.unlocked ? 'bg-emerald-500' : 'bg-primary-500'"
                        :style="{ width: `${row.progressPercent}%` }"
                      ></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="grid gap-4 xl:grid-cols-[1.12fr_0.88fr]">
          <div class="card p-6 xl:col-span-2">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="text-base font-semibold text-terra-ink dark:text-white">
                  {{ t('affiliate.registrationSeats.title') }}
                </h3>
                <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">
                  {{ t('affiliate.registrationSeats.description') }}
                </p>
              </div>
              <div
                class="inline-flex w-fit items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold"
                :class="seatLinkUsable
                  ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-200 dark:ring-emerald-800/50'
                  : 'bg-amber-50 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-900/25 dark:text-amber-200 dark:ring-amber-800/50'"
              >
                <Icon :name="seatLinkUsable ? 'checkCircle' : 'exclamationTriangle'" size="sm" />
                <span>{{ seatLinkUsable ? t('affiliate.registrationSeats.statusAvailable') : t('affiliate.registrationSeats.statusEmpty') }}</span>
              </div>
            </div>

            <div v-if="registrationSeatsFree" class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/50 dark:bg-emerald-900/20">
              <div class="flex items-start gap-3">
                <div class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                  <Icon name="checkCircle" size="sm" />
                </div>
                <div>
                  <p class="text-sm font-semibold text-emerald-800 dark:text-emerald-100">
                    {{ t('affiliate.registrationSeats.freeModeTitle') }}
                  </p>
                  <p class="mt-1 text-sm text-emerald-700 dark:text-emerald-200">
                    {{ t('affiliate.registrationSeats.freeModeDescription') }}
                  </p>
                </div>
              </div>
            </div>

            <div v-else class="mt-5 grid gap-3 sm:grid-cols-3">
              <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
                <p class="text-xs font-medium text-terra-muted dark:text-dark-400">
                  {{ t('affiliate.registrationSeats.available') }}
                </p>
                <p class="mt-1 text-2xl font-semibold text-emerald-700 dark:text-emerald-300">
                  {{ formatCount(seatStats.available) }}
                </p>
              </div>
              <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
                <p class="text-xs font-medium text-terra-muted dark:text-dark-400">
                  {{ t('affiliate.registrationSeats.used') }}
                </p>
                <p class="mt-1 text-2xl font-semibold text-terra-ink dark:text-dark-100">
                  {{ formatCount(seatStats.used) }}
                </p>
              </div>
              <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
                <p class="text-xs font-medium text-terra-muted dark:text-dark-400">
                  {{ t('affiliate.registrationSeats.total') }}
                </p>
                <p class="mt-1 text-2xl font-semibold text-terra-ink dark:text-dark-100">
                  {{ formatCount(seatStats.total) }}
                </p>
              </div>
            </div>

            <div v-if="!registrationSeatsFree" class="mt-5 rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70">
              <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
                <div class="grid gap-3 sm:grid-cols-[minmax(150px,220px)_minmax(0,1fr)]">
                  <div>
                    <label for="registration-seat-quantity" class="text-xs font-semibold text-terra-muted dark:text-dark-400">
                      {{ t('affiliate.registrationSeats.quantity') }}
                    </label>
                    <input
                      id="registration-seat-quantity"
                      v-model.number="seatQuantity"
                      type="number"
                      min="1"
                      max="1000"
                      step="1"
                      class="mt-2 h-10 w-full rounded-lg border border-terra-line bg-white px-3 text-sm font-semibold text-terra-ink outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-dark-700 dark:bg-dark-800 dark:text-white"
                      @blur="normalizeSeatQuantity"
                    />
                  </div>
                  <div class="grid gap-2 sm:grid-cols-2">
                    <div>
                      <p class="text-xs font-semibold text-terra-muted dark:text-dark-400">
                        {{ t('affiliate.registrationSeats.cost') }}
                      </p>
                      <p class="mt-2 text-sm font-semibold text-terra-ink dark:text-dark-100">
                        {{ formatCurrency(seatCost) }}
                      </p>
                    </div>
                    <div>
                      <p class="text-xs font-semibold text-terra-muted dark:text-dark-400">
                        {{ t('affiliate.registrationSeats.expectedCost') }}
                      </p>
                      <p class="mt-2 text-sm font-semibold text-primary-700 dark:text-primary-300">
                        {{ formatCurrency(seatExpectedCost) }}
                      </p>
                    </div>
                  </div>
                </div>

                <button
                  type="button"
                  class="btn btn-primary justify-center"
                  :disabled="purchasingSeats || !canPurchaseSeats"
                  @click="purchaseRegistrationSeats"
                >
                  <Icon v-if="purchasingSeats" name="refresh" size="sm" class="animate-spin" />
                  <Icon v-else name="plus" size="sm" />
                  <span>{{ purchasingSeats ? t('affiliate.registrationSeats.purchasing') : t('affiliate.registrationSeats.purchase') }}</span>
                </button>
              </div>
              <p class="mt-3 text-xs text-terra-muted dark:text-dark-400">
                {{ t('affiliate.registrationSeats.balanceHint', { balance: formatCurrency(currentBalance) }) }}
              </p>
            </div>
          </div>

        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="transferring || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="dollar" size="sm" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>
          <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('affiliate.transfer.empty') }}
          </p>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h3 class="text-base font-semibold text-terra-ink dark:text-white">
                {{ t('affiliate.levelDetails.title') }}
              </h3>
              <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">
                {{ t('affiliate.levelDetails.subtitle') }}
              </p>
            </div>

            <div class="grid gap-2 rounded-lg border border-terra-line bg-terra-surface p-1 dark:border-dark-700 dark:bg-dark-900 sm:grid-cols-3">
              <button
                v-for="item in levelDetails"
                :key="item.level"
                type="button"
                class="min-w-0 rounded-md px-3 py-2 text-left transition"
                :class="activeLevel === item.level
                  ? 'bg-terra-elevated text-primary-700 shadow-glass-sm dark:bg-dark-800 dark:text-primary-200'
                  : 'text-terra-muted hover:bg-terra-elevated/70 dark:text-dark-300 dark:hover:bg-dark-800/60'"
                @click="activeLevel = item.level"
              >
                <span class="block text-xs font-semibold">
                  {{ t('affiliate.levelDetails.levelTab', { level: item.level }) }}
                </span>
                <span class="mt-0.5 block truncate text-sm font-semibold">
                  {{ formatCurrency(item.total_rebate) }}
                </span>
              </button>
            </div>
          </div>

          <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.invitees') }}</p>
              <p class="mt-1 text-lg font-semibold text-terra-ink dark:text-white">{{ formatCount(selectedLevelDetail.invitee_count) }}</p>
            </div>
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.total') }}</p>
              <p class="mt-1 text-lg font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(selectedLevelDetail.total_rebate) }}</p>
            </div>
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.frozen') }}</p>
              <p class="mt-1 text-lg font-semibold text-accent-600 dark:text-accent-300">{{ formatCurrency(selectedLevelDetail.frozen_rebate) }}</p>
            </div>
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.available') }}</p>
              <p class="mt-1 text-lg font-semibold text-emerald-700 dark:text-emerald-300">{{ formatCurrency(selectedLevelDetail.available_rebate) }}</p>
            </div>
          </div>

          <div v-if="selectedLevelDetail.invitees.length === 0" class="mt-5 rounded-lg border border-dashed border-terra-line p-6 text-center text-sm text-terra-muted dark:border-dark-700 dark:text-dark-400">
            {{ t('affiliate.levelDetails.empty', { level: selectedLevelLabel }) }}
          </div>

          <template v-else>
            <div class="mt-5 hidden overflow-x-auto md:block">
              <table class="w-full min-w-[920px] text-left text-sm">
                <thead>
                  <tr class="border-b border-terra-line text-xs uppercase tracking-wide text-terra-muted dark:border-dark-700 dark:text-dark-400">
                    <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.user') }}</th>
                    <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.chain') }}</th>
                    <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.joinedAt') }}</th>
                    <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.orders') }}</th>
                    <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.total') }}</th>
                    <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.frozen') }}</th>
                    <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.available') }}</th>
                    <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.lastRebate') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in selectedLevelDetail.invitees"
                    :key="`${selectedLevelDetail.level}-${item.user_id}`"
                    class="border-b border-terra-line/70 last:border-b-0 dark:border-dark-800"
                  >
                    <td class="px-3 py-3">
                      <p class="max-w-[220px] truncate font-medium text-terra-ink dark:text-white">
                        {{ item.email || fallbackUserLabel(item) }}
                      </p>
                      <p class="max-w-[220px] truncate text-xs text-terra-muted dark:text-dark-400">
                        {{ item.username || fallbackUserLabel(item) }}
                      </p>
                    </td>
                    <td class="px-3 py-3 text-terra-muted dark:text-dark-300">
                      <span class="inline-flex max-w-[180px] items-center rounded-md bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">
                        <span class="truncate">{{ chainLabel(item, selectedLevelDetail.level) }}</span>
                      </span>
                    </td>
                    <td class="px-3 py-3 text-terra-muted dark:text-dark-300">{{ formatDateTime(item.joined_at) || '-' }}</td>
                    <td class="px-3 py-3 text-right text-terra-ink dark:text-dark-100">{{ formatOrderCount(item.order_count) }}</td>
                    <td class="px-3 py-3 text-right font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(item.total_rebate) }}</td>
                    <td class="px-3 py-3 text-right font-medium text-accent-600 dark:text-accent-300">{{ formatCurrency(item.frozen_rebate) }}</td>
                    <td class="px-3 py-3 text-right font-medium text-emerald-700 dark:text-emerald-300">{{ formatCurrency(item.available_rebate) }}</td>
                    <td class="px-3 py-3 text-terra-muted dark:text-dark-300">{{ formatDateTime(item.last_rebate_at) || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="mt-5 space-y-3 md:hidden">
              <div
                v-for="item in selectedLevelDetail.invitees"
                :key="`${selectedLevelDetail.level}-mobile-${item.user_id}`"
                class="rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate font-medium text-terra-ink dark:text-white">
                      {{ item.email || fallbackUserLabel(item) }}
                    </p>
                    <p class="mt-0.5 truncate text-xs text-terra-muted dark:text-dark-400">
                      {{ chainLabel(item, selectedLevelDetail.level) }}
                    </p>
                  </div>
                  <p class="shrink-0 text-sm font-semibold text-primary-700 dark:text-primary-300">
                    {{ formatCurrency(item.total_rebate) }}
                  </p>
                </div>
                <div class="mt-3 grid grid-cols-2 gap-3 text-xs">
                  <div>
                    <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.orders') }}</p>
                    <p class="mt-0.5 font-medium text-terra-ink dark:text-dark-100">{{ formatOrderCount(item.order_count) }}</p>
                  </div>
                  <div>
                    <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.available') }}</p>
                    <p class="mt-0.5 font-medium text-emerald-700 dark:text-emerald-300">{{ formatCurrency(item.available_rebate) }}</p>
                  </div>
                  <div>
                    <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.frozen') }}</p>
                    <p class="mt-0.5 font-medium text-accent-600 dark:text-accent-300">{{ formatCurrency(item.frozen_rebate) }}</p>
                  </div>
                  <div>
                    <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.lastRebate') }}</p>
                    <p class="mt-0.5 font-medium text-terra-ink dark:text-dark-100">{{ formatDateTime(item.last_rebate_at) || '-' }}</p>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>
      </template>
    </div>

    <Teleport to="body">
      <div
        v-if="tutorialDialogOpen"
        class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 p-4 backdrop-blur-sm"
        @click.self="closeGrowthTutorial"
      >
        <div class="w-full max-w-lg rounded-2xl bg-white shadow-2xl dark:bg-dark-900">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">邀请返利教学</h3>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">了解规则即可完成新人任务</p>
            </div>
            <button
              type="button"
              class="rounded-lg p-2 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-800 dark:hover:text-dark-200"
              @click="closeGrowthTutorial"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
          </div>
          <div class="space-y-3 p-5 text-sm text-gray-600 dark:text-dark-300">
            <div class="rounded-xl border border-primary-100 bg-primary-50 p-4 dark:border-primary-900/50 dark:bg-primary-900/20">
              <p class="font-semibold text-primary-800 dark:text-primary-200">你的邀请链接在本页上方；规则和升级进度已在本页集中展示。</p>
              <p class="mt-2 leading-6 text-primary-700 dark:text-primary-300">
                一级返利默认生效；二级、三级会按有效邀请人数自动解锁，只有完成真实支付的直属用户才计入进度。
              </p>
            </div>
            <div class="grid gap-3 sm:grid-cols-3">
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">第 1 步</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">复制链接</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">第 2 步</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">邀请注册</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">第 3 步</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">充值返利</p>
              </div>
            </div>
          </div>
          <div class="flex justify-end gap-3 border-t border-gray-100 px-5 py-4 dark:border-dark-700">
            <button type="button" class="btn btn-secondary" @click="closeGrowthTutorial">稍后</button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="confirmingGrowthTutorial"
              @click="confirmGrowthTutorial"
            >
              <Icon v-if="confirmingGrowthTutorial" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="check" size="sm" />
              <span>{{ confirmingGrowthTutorial ? '确认中' : '我已了解' }}</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { AffiliateLevelDetail, AffiliateLevelInvitee, AffiliateLevelRateRule, UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const purchasingSeats = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)
const activeLevel = ref(1)
const seatQuantity = ref(1)
const tutorialDialogOpen = ref(false)
const confirmingGrowthTutorial = ref(false)

const fallbackLevelUnlockThresholds: Record<number, number> = {
  1: 0,
  2: 3,
  3: 10,
}

interface RebateMechanismItem {
  level: number
  title: string
  description: string
  statusLabel: string
  rateLabel: string
  unlocked: boolean
}

interface UnlockProgressRow {
  level: number
  title: string
  source: string
  rateLabel: string
  unlocked: boolean
  remaining: number
  progressPercent: number
  progressLabel: string
  progressText: string
  requirementLabel: string
  statusLabel: string
}

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

const levelRebateSummaries = computed(() => {
  const rows = detail.value?.level_rebates ?? []
  return [1, 2, 3].map((level) => ({
    level,
    rebate_amount: rows.find((item) => item.level === level)?.rebate_amount ?? 0,
  }))
})

const seatStats = computed(() => ({
  total: detail.value?.registration_seat_total ?? 0,
  used: detail.value?.registration_seat_used ?? 0,
  available: detail.value?.registration_seat_available ?? 0,
}))

const seatCost = computed(() => detail.value?.registration_seat_cost ?? 0)
const registrationSeatsFree = computed(() => seatCost.value <= 0)
const normalizedSeatQuantityValue = computed(() => normalizeQuantityValue(seatQuantity.value))
const seatExpectedCost = computed(() => normalizedSeatQuantityValue.value * seatCost.value)
const currentBalance = computed(() => authStore.user?.balance ?? 0)
const seatLinkUsable = computed(() => registrationSeatsFree.value || seatStats.value.available > 0)
const canPurchaseSeats = computed(() => !registrationSeatsFree.value && normalizedSeatQuantityValue.value > 0 && normalizedSeatQuantityValue.value <= 1000)
const qualifiedInviteCount = computed(() => detail.value?.qualified_aff_count ?? 0)

const effectiveLevelRates = computed<AffiliateLevelRateRule[]>(() => {
  const rows = detail.value?.effective_level_rates ?? []
  const invitedCount = qualifiedInviteCount.value
  return [1, 2, 3].map((level) => {
    const existing = rows.find((item) => item.level === level)
    const threshold = normalizeUnlockThreshold(existing?.unlock_invite_count ?? fallbackLevelUnlockThresholds[level] ?? 0)
    const unlocked = existing?.unlocked ?? (level <= 1 || invitedCount >= threshold)
    return {
      level,
      rate_percent: existing?.rate_percent ?? (level === 1 ? (detail.value?.effective_rebate_rate_percent ?? 0) : 0),
      source: existing?.source ?? (unlocked ? 'global' : 'locked'),
      unlocked,
      unlock_invite_count: threshold,
    }
  })
})

const level3UnlockThreshold = computed(() => levelUnlockThreshold(3))

const unlockProgressRows = computed<UnlockProgressRow[]>(() => {
  const invitedCount = qualifiedInviteCount.value
  return effectiveLevelRates.value.map((rule) => {
    const threshold = normalizeUnlockThreshold(rule.unlock_invite_count ?? fallbackLevelUnlockThresholds[rule.level] ?? 0)
    const unlocked = isLevelUnlocked(rule.level)
    const remaining = Math.max(0, threshold - invitedCount)
    const progressPercent = rule.level <= 1 || threshold <= 0
      ? 100
      : Math.min(100, Math.max(0, (invitedCount / threshold) * 100))
    const cappedProgressCount = rule.level <= 1 ? 1 : Math.min(invitedCount, threshold)

    return {
      level: rule.level,
      title: t('affiliate.levelRules.levelTitle', { level: rule.level }),
      source: rule.source,
      rateLabel: unlocked
        ? t('affiliate.rebateGuide.rateValue', { rate: `${formatPercent(rule.rate_percent)}%` })
        : t('affiliate.rebateGuide.lockedRateLabel'),
      unlocked,
      remaining,
      progressPercent,
      progressLabel: rule.level <= 1
        ? t('affiliate.rebateGuide.alwaysOnProgress')
        : t('affiliate.rebateGuide.inviteProgress'),
      progressText: rule.level <= 1
        ? t('affiliate.rebateGuide.availableNow')
        : t('affiliate.rebateGuide.progressValue', {
            current: formatCount(cappedProgressCount),
            target: formatCount(threshold),
          }),
      requirementLabel: rule.level <= 1
        ? t('affiliate.rebateGuide.level1Requirement')
        : t('affiliate.rebateGuide.levelUnlockRequirement', {
            level: rule.level,
            count: formatCount(threshold),
          }),
      statusLabel: unlocked
        ? t('affiliate.rebateGuide.unlocked')
        : t('affiliate.rebateGuide.locked'),
    }
  })
})

const nextLockedProgress = computed(() => unlockProgressRows.value.find((row) => !row.unlocked) ?? null)

const rebateMechanismItems = computed<RebateMechanismItem[]>(() => (
  unlockProgressRows.value.map((row) => ({
    level: row.level,
    title: rebateMechanismTitle(row.level),
    description: rebateMechanismDescription(row.level),
    statusLabel: row.statusLabel,
    rateLabel: row.rateLabel,
    unlocked: row.unlocked,
  }))
))

const rebateFlowItems = computed(() => [
  t('affiliate.rebateGuide.flowInvite'),
  t('affiliate.rebateGuide.flowRecharge'),
  t('affiliate.rebateGuide.flowFreeze'),
  t('affiliate.rebateGuide.flowTransfer'),
])

const levelDetails = computed<AffiliateLevelDetail[]>(() => {
  const rows = detail.value?.level_details ?? []
  return [1, 2, 3].map((level) => {
    const existing = rows.find((item) => item.level === level)
    if (existing) {
      return {
        ...existing,
        invitees: existing.invitees ?? []
      }
    }

    const fallbackInvitees: AffiliateLevelInvitee[] = level === 1
      ? (detail.value?.invitees ?? []).map((item) => ({
          user_id: item.user_id,
          email: item.email,
          username: item.username,
          joined_at: item.created_at,
          total_rebate: item.total_rebate,
          frozen_rebate: 0,
          available_rebate: item.total_rebate,
          order_count: 0,
        }))
      : []
    const rebateAmount = levelRebateSummaries.value.find((item) => item.level === level)?.rebate_amount ?? 0

    return {
      level,
      invitee_count: fallbackInvitees.length,
      total_rebate: rebateAmount,
      frozen_rebate: 0,
      available_rebate: rebateAmount,
      invitees: fallbackInvitees,
    }
  })
})

const selectedLevelDetail = computed<AffiliateLevelDetail>(() => (
  levelDetails.value.find((item) => item.level === activeLevel.value) ?? levelDetails.value[0]
))

const selectedLevelLabel = computed(() => (
  t('affiliate.levelDetails.levelLabel', { level: selectedLevelDetail.value.level })
))

function formatCount(value: number): string {
  return value.toLocaleString()
}

function formatPercent(value: number): string {
  const rounded = Math.round(value * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
}

function formatOrderCount(value: number): string {
  return t('affiliate.levelDetails.orderCount', { count: formatCount(value) })
}

function normalizeUnlockThreshold(value: number): number {
  const parsed = Math.floor(Number(value))
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

function levelUnlockThreshold(level: number): number {
  const rule = effectiveLevelRates.value.find((item) => item.level === level)
  return normalizeUnlockThreshold(rule?.unlock_invite_count ?? fallbackLevelUnlockThresholds[level] ?? 0)
}

function isLevelUnlocked(level: number): boolean {
  if (level <= 1) return true
  const rule = effectiveLevelRates.value.find((item) => item.level === level)
  if (typeof rule?.unlocked === 'boolean') {
    return rule.unlocked
  }
  const threshold = levelUnlockThreshold(level)
  return threshold > 0 && qualifiedInviteCount.value >= threshold
}

function normalizeQuantityValue(value: number): number {
  const parsed = Math.floor(Number(value))
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }
  return Math.min(parsed, 1000)
}

function normalizeSeatQuantity(): void {
  const normalized = normalizeQuantityValue(seatQuantity.value)
  seatQuantity.value = normalized > 0 ? normalized : 1
}

function levelRateSourceLabel(source: string): string {
  if (source === 'exclusive') {
    return t('affiliate.levelRules.sourceExclusive')
  }
  if (source === 'locked') {
    return t('affiliate.levelRules.sourceLocked')
  }
  return t('affiliate.levelRules.sourceGlobal')
}

function rebateMechanismTitle(level: number): string {
  switch (level) {
    case 1:
      return t('affiliate.rebateGuide.level1Title')
    case 2:
      return t('affiliate.rebateGuide.level2Title')
    case 3:
      return t('affiliate.rebateGuide.level3Title')
    default:
      return t('affiliate.levelRules.levelTitle', { level })
  }
}

function rebateMechanismDescription(level: number): string {
  switch (level) {
    case 1:
      return t('affiliate.rebateGuide.level1Description')
    case 2:
      return t('affiliate.rebateGuide.level2Description')
    case 3:
      return t('affiliate.rebateGuide.level3Description')
    default:
      return ''
  }
}

function fallbackUserLabel(item: AffiliateLevelInvitee): string {
  return `#${item.user_id}`
}

function displayParent(item: AffiliateLevelInvitee): string {
  return item.parent_email || item.parent_username || (item.parent_user_id ? `#${item.parent_user_id}` : '')
}

function chainLabel(item: AffiliateLevelInvitee, level: number): string {
  if (level <= 1) {
    return t('affiliate.levelDetails.directChain')
  }
  const parent = displayParent(item)
  return parent
    ? t('affiliate.levelDetails.viaUser', { user: parent })
    : t('affiliate.levelDetails.noChain')
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

async function purchaseRegistrationSeats(): Promise<void> {
  normalizeSeatQuantity()
  const quantity = normalizeQuantityValue(seatQuantity.value)
  if (quantity <= 0 || purchasingSeats.value) {
    appStore.showError(t('affiliate.registrationSeats.invalidQuantity'))
    return
  }

  purchasingSeats.value = true
  const expectedCost = seatExpectedCost.value
  try {
    const resp = await userAPI.purchaseAffiliateRegistrationSeats(quantity)
    if (detail.value) {
      detail.value.registration_seat_cost = resp.registration_seat_cost
      detail.value.registration_seat_total = resp.registration_seat_total
      detail.value.registration_seat_used = resp.registration_seat_used
      detail.value.registration_seat_available = resp.registration_seat_available
    }
    appStore.showSuccess(t('affiliate.registrationSeats.purchaseSuccess', {
      count: formatCount(quantity),
      amount: formatCurrency(expectedCost),
    }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.registrationSeats.purchaseFailed')))
  } finally {
    purchasingSeats.value = false
  }
}

function maybeOpenGrowthTutorial(): void {
  if (route.query.growth_tutorial === '1') {
    tutorialDialogOpen.value = true
  }
}

function closeGrowthTutorial(): void {
  tutorialDialogOpen.value = false
}

async function confirmGrowthTutorial(): Promise<void> {
  if (confirmingGrowthTutorial.value) return
  confirmingGrowthTutorial.value = true
  try {
    await userAPI.markAffiliateTutorialDone()
    tutorialDialogOpen.value = false
    window.dispatchEvent(new CustomEvent('growth:center-refresh'))
    appStore.showSuccess('新人任务已完成')
    const query = { ...route.query }
    delete query.growth_tutorial
    await router.replace({ query })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '确认失败，请稍后重试'))
  } finally {
    confirmingGrowthTutorial.value = false
  }
}

watch(() => route.query.growth_tutorial, () => {
  maybeOpenGrowthTutorial()
})

onMounted(() => {
  void loadAffiliateDetail()
  maybeOpenGrowthTutorial()
})
</script>
