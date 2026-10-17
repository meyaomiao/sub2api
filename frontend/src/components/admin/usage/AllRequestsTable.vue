<template>
  <div class="overflow-x-auto">
    <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
      <thead class="bg-gray-50 dark:bg-dark-800">
        <tr>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.time') }}
          </th>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.kind') }}
          </th>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.platform') }}
          </th>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.model') }}
          </th>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.status') }}
          </th>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.duration') }}
          </th>
          <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.requestId') }}
          </th>
          <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestDetails.table.actions') }}
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
        <tr v-if="loading && (!items || items.length === 0)">
          <td colspan="8" class="p-8 text-center text-sm text-gray-500 dark:text-gray-400">
            <div class="flex items-center justify-center gap-2">
              <Icon name="refresh" class="animate-spin" size="sm" />
              <span>{{ t('common.loading') }}</span>
            </div>
          </td>
        </tr>
        <tr v-else-if="!items || items.length === 0">
          <td colspan="8" class="p-8 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('common.noData') }}
          </td>
        </tr>
        <tr
          v-for="row in items"
          :key="row.request_id || row.created_at"
          class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/60"
        >
          <!-- Time -->
          <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
            {{ formatDateTime(row.created_at) }}
          </td>

          <!-- Kind Badge (Success / Error) -->
          <td class="whitespace-nowrap px-4 py-3">
            <span
              v-if="row.kind === 'error'"
              class="inline-flex items-center rounded-full bg-red-100 px-2 py-0.5 text-[11px] font-semibold text-red-700 dark:bg-red-900/30 dark:text-red-300"
            >
              {{ t('admin.ops.requestDetails.kind.error') }}
            </span>
            <span
              v-else
              class="inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
            >
              {{ t('admin.ops.requestDetails.kind.success') }}
            </span>
          </td>

          <!-- Platform -->
          <td class="whitespace-nowrap px-4 py-3 text-xs font-medium text-gray-700 dark:text-gray-200">
            {{ (row.platform || '-').toUpperCase() }}
          </td>

          <!-- Model -->
          <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-900 dark:text-white font-mono">
            {{ row.model || '-' }}
          </td>

          <!-- Status Code -->
          <td class="whitespace-nowrap px-4 py-3 text-xs">
            <span
              v-if="row.kind === 'error'"
              class="inline-flex items-center rounded px-1.5 py-0.5 font-mono text-[11px] font-bold"
              :class="getStatusClass(row.status_code)"
            >
              {{ row.status_code || '-' }}
            </span>
            <span
              v-else
              class="inline-flex items-center rounded bg-emerald-50 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400"
            >
              200
            </span>
          </td>

          <!-- Duration -->
          <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500 dark:text-gray-400 font-mono">
            {{ row.duration_ms != null ? `${row.duration_ms} ms` : '-' }}
          </td>

          <!-- Request ID -->
          <td class="whitespace-nowrap px-4 py-3 text-xs font-mono text-gray-500 dark:text-gray-400">
            <span class="truncate max-w-[150px] inline-block" :title="row.request_id">
              {{ row.request_id || '-' }}
            </span>
          </td>

          <!-- Action -->
          <td class="whitespace-nowrap px-4 py-3 text-right text-xs">
            <button
              v-if="row.kind === 'error' && row.error_id"
              @click="$emit('open-error-detail', row.error_id)"
              class="inline-flex items-center gap-1 font-medium text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300"
            >
              <span>{{ t('admin.ops.requestDetails.viewError') }}</span>
              <Icon name="chevronRight" size="xs" />
            </button>
            <span v-else class="text-gray-400">-</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type { OpsRequestDetail } from '@/api/admin/ops'

interface Props {
  items: OpsRequestDetail[]
  loading: boolean
}

defineProps<Props>()

defineEmits<{
  (e: 'open-error-detail', errorId: number): void
}>()

const { t } = useI18n()

function getStatusClass(statusCode?: number | null): string {
  if (!statusCode) return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
  if (statusCode >= 500) return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300'
  if (statusCode >= 400) return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300'
}
</script>
