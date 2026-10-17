<template>
  <div
    class="relative flex flex-col justify-between rounded-xl border bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md dark:border-dark-700 dark:bg-dark-800"
    :class="[
      isSelected ? 'border-primary-500 ring-2 ring-primary-500/20' : 'border-gray-200 dark:border-dark-700',
      !account.schedulable ? 'opacity-75 bg-gray-50/50 dark:bg-dark-800/50' : ''
    ]"
  >
    <!-- Top Header: Selection + Account Name/ID + Schedulable Switch -->
    <div>
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-2 min-w-0 flex-1">
          <!-- Checkbox -->
          <input
            type="checkbox"
            :checked="isSelected"
            @change="$emit('toggle-select', account.id)"
            class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />

          <!-- Account Name -->
          <div class="flex items-center gap-1.5 min-w-0 flex-1">
            <span class="font-mono text-xs text-gray-400 dark:text-gray-500 shrink-0">#{{ account.id }}</span>
            <a
              v-if="homepageUrl"
              :href="homepageUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="truncate font-semibold text-gray-900 hover:underline dark:text-white text-sm"
              :title="account.name"
            >
              {{ account.name }}
            </a>
            <span
              v-else
              class="truncate font-semibold text-gray-900 dark:text-white text-sm"
              :title="account.name"
            >
              {{ account.name }}
            </span>
          </div>
        </div>

        <!-- Schedulable Toggle -->
        <div class="flex items-center gap-1.5 shrink-0">
          <button
            @click="$emit('toggle-schedulable', account)"
            :disabled="togglingSchedulable"
            class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:focus:ring-offset-dark-800"
            :class="[account.schedulable ? 'bg-primary-500 hover:bg-primary-600' : 'bg-gray-200 hover:bg-gray-300 dark:bg-dark-600 dark:hover:bg-dark-500']"
            :title="account.schedulable ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled')"
          >
            <span
              class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="[account.schedulable ? 'translate-x-4' : 'translate-x-0']"
            />
          </button>
        </div>
      </div>

      <!-- Email Subtitle -->
      <div v-if="displayEmail" class="mt-1 pl-6">
        <span
          class="block truncate text-xs text-gray-500 dark:text-gray-400"
          :title="displayEmail"
        >
          {{ displayEmail }}
        </span>
      </div>

      <!-- Platform, Type & Status Badges -->
      <div class="mt-2.5 flex flex-wrap items-center gap-1.5 pl-6">
        <PlatformTypeBadge
          :platform="account.platform"
          :type="account.type"
          :auth-mode="openAIAuthMode"
          :plan-type="planType"
          :privacy-mode="privacyModeString"
          :subscription-expires-at="subscriptionExpiresAtString"
        />
        <span
          v-if="antigravityTierLabel"
          :class="['inline-block rounded px-1.5 py-0.5 text-[10px] font-medium', antigravityTierClass]"
        >
          {{ antigravityTierLabel }}
        </span>

        <!-- Status Indicator -->
        <div class="scale-90 origin-left">
          <AccountStatusIndicator :account="account" @show-temp-unsched="$emit('show-temp-unsched', account)" />
        </div>

        <!-- Multiplier -->
        <span v-if="account.rate_multiplier && account.rate_multiplier !== 1" class="rounded bg-gray-100 dark:bg-dark-700 px-1.5 py-0.5 text-[10px] font-mono font-medium text-gray-600 dark:text-gray-300">
          {{ account.rate_multiplier }}x
        </span>
      </div>
    </div>

    <!-- Usage Windows (Progress Bars) Container -->
    <div class="my-3 rounded-lg border border-gray-100 bg-gray-50/70 p-2.5 dark:border-dark-700 dark:bg-dark-700/30">
      <div class="text-[11px] font-medium text-gray-500 dark:text-gray-400 mb-1.5 flex items-center justify-between">
        <span>{{ t('admin.accounts.columns.usageWindows') }}</span>
        <span v-if="account.proxy" class="text-[10px] text-gray-400 truncate max-w-[120px]" :title="account.proxy.name">
          🌐 {{ account.proxy.country_code || account.proxy.name }}
        </span>
      </div>
      <AccountUsageCell
        :account="account"
        :today-stats="todayStats"
        :today-stats-loading="todayStatsLoading"
        :manual-refresh-token="manualRefreshToken"
        :batched-usage="batchedUsage"
        :batched-usage-error="batchedUsageError"
        :batched-usage-loading="batchedUsageLoading"
        :request-batched-usage="requestBatchedUsage"
        @account-updated="$emit('account-updated', $event)"
        @usage-loaded="$emit('usage-loaded', account.id, $event)"
      />
    </div>

    <!-- Metadata Section: Today Stats, Groups, Notes -->
    <div class="space-y-2 text-xs">
      <!-- Today Stats -->
      <div class="flex items-center justify-between gap-2 px-1">
        <span class="text-[11px] text-gray-400 shrink-0">{{ t('admin.accounts.columns.todayStats') }}:</span>
        <div class="flex items-center gap-2 overflow-hidden">
          <AccountTodayStatsCell
            :stats="todayStats"
            :loading="todayStatsLoading"
            :error="todayStatsError"
          />
          <AccountCapacityCell :account="account" />
        </div>
      </div>

      <!-- Groups Tag -->
      <div v-if="groups.length" class="flex items-center gap-1.5 px-1 overflow-hidden">
        <span class="text-[11px] text-gray-400 shrink-0">{{ t('admin.accounts.columns.groups') }}:</span>
        <AccountGroupsCell :groups="groups" :max-display="3" />
      </div>

      <!-- Notes -->
      <div v-if="account.notes" class="truncate text-[11px] text-gray-400 dark:text-gray-500 px-1" :title="account.notes">
        💬 {{ account.notes }}
      </div>
    </div>

    <!-- Card Footer: Last Used + Actions -->
    <div class="mt-3 flex items-center justify-between border-t border-gray-100 pt-2.5 dark:border-dark-700/60">
      <div class="text-[11px] text-gray-400">
        {{ account.last_used_at ? formatRelativeTime(account.last_used_at) : '-' }}
      </div>
      <div class="flex items-center gap-1">
        <button
          @click="$emit('test', account)"
          class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700 transition"
          :title="t('admin.accounts.test')"
        >
          <Icon name="bolt" size="xs" />
          <span>{{ t('admin.accounts.test') }}</span>
        </button>
        <button
          @click="$emit('edit', account)"
          class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-primary-900/30 transition"
          :title="t('common.edit')"
        >
          <Icon name="edit" size="xs" />
          <span>{{ t('common.edit') }}</span>
        </button>
        <button
          @click="$emit('open-menu', account, $event)"
          class="inline-flex items-center justify-center rounded p-1 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200 transition"
        >
          <Icon name="more" size="xs" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountListItem } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountUsageCell from '@/components/account/AccountUsageCell.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import { formatRelativeTime } from '@/utils/format'

interface Props {
  account: AccountListItem
  isSelected: boolean
  togglingSchedulable?: boolean
  todayStats?: any
  todayStatsLoading?: boolean
  todayStatsError?: string | null
  manualRefreshToken?: number
  batchedUsage?: any
  batchedUsageError?: any
  batchedUsageLoading?: boolean
  requestBatchedUsage?: any
  homepageUrl?: string | null
  displayEmail?: string
  openAIAuthMode?: string
  planType?: string
  antigravityTierLabel?: string
  antigravityTierClass?: string
  groups?: any[]
}

const props = withDefaults(defineProps<Props>(), {
  groups: () => []
})

const privacyModeString = computed<string | undefined>(() => {
  const mode = props.account.extra?.privacy_mode || props.account.parent_privacy_mode
  return typeof mode === 'string' ? mode : undefined
})

const subscriptionExpiresAtString = computed<string | undefined>(() => {
  const expiresAt = props.account.credentials?.subscription_expires_at || props.account.parent_subscription_expires_at
  return typeof expiresAt === 'string' ? expiresAt : undefined
})

defineEmits<{
  (e: 'toggle-select', id: number): void
  (e: 'toggle-schedulable', account: AccountListItem): void
  (e: 'show-temp-unsched', account: AccountListItem): void
  (e: 'test', account: AccountListItem): void
  (e: 'edit', account: AccountListItem): void
  (e: 'open-menu', account: AccountListItem, event: MouseEvent): void
  (e: 'account-updated', account: Account): void
  (e: 'usage-loaded', id: number, event: any): void
}>()

const { t } = useI18n()
</script>
