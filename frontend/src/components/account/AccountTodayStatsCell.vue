<template>
  <div>
    <!-- Loading state -->
    <div v-if="props.loading && !props.stats" class="space-y-0.5">
      <div class="h-3 w-12 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
      <div class="h-3 w-16 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
      <div class="h-3 w-10 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    </div>

    <!-- Error state -->
    <div v-else-if="props.error && !props.stats" class="text-xs text-red-500">
      {{ props.error }}
    </div>

    <!-- Stats data -->
    <div v-else-if="props.stats" class="inline-flex items-center gap-1.5 text-xs font-mono">
      <span class="inline-flex items-center gap-1 rounded bg-gray-100/80 px-2 py-0.5 text-gray-600 dark:bg-dark-700 dark:text-gray-300">
        <span class="text-[10px] text-gray-400 font-sans">{{ t('admin.accounts.stats.requests') }}</span>
        <span class="font-semibold">{{ formatNumber(props.stats.requests) }}</span>
      </span>
      <span class="inline-flex items-center gap-1 rounded bg-gray-100/80 px-2 py-0.5 text-gray-600 dark:bg-dark-700 dark:text-gray-300">
        <span class="text-[10px] text-gray-400 font-sans">{{ t('admin.accounts.stats.tokens') }}</span>
        <span class="font-semibold">{{ formatTokens(props.stats.tokens) }}</span>
      </span>
    </div>

    <!-- No data -->
    <div v-else class="text-xs text-gray-400">-</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { WindowStats } from '@/types'
import { formatNumber } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    stats?: WindowStats | null
    loading?: boolean
    error?: string | null
  }>(),
  {
    stats: null,
    loading: false,
    error: null
  }
)

const { t } = useI18n()

// Format large token numbers (e.g., 1234567 -> 1.23M)
const formatTokens = (tokens: number): string => {
  if (tokens >= 1000000) {
    return `${(tokens / 1000000).toFixed(2)}M`
  } else if (tokens >= 1000) {
    return `${(tokens / 1000).toFixed(1)}K`
  }
  return tokens.toString()
}
</script>
