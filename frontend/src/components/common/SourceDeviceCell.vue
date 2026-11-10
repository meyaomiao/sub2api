<template>
  <div v-if="ip && device" :title="deviceTitle" data-testid="usage-ip-device">
    <div class="flex items-center gap-1.5 text-sm font-medium text-gray-800 dark:text-gray-200">
      <span
        class="inline-block h-1.5 w-1.5 shrink-0 rounded-full"
        :class="device.online ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-500'"
      ></span>
      <span class="max-w-[180px] truncate">{{ device.name }}</span>
    </div>
    <span class="text-xs font-mono text-gray-400 dark:text-gray-500">{{ ip }}</span>
  </div>
  <div v-else-if="ip">
    <span class="text-sm font-mono text-gray-600 dark:text-gray-400">{{ ip }}</span>
    <IpGeoCell :ip="ip" />
  </div>
  <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { IpDevice } from '@/api/admin/usage'
import IpGeoCell from '@/components/common/IpGeoCell.vue'

const props = defineProps<{
  ip?: string | null
  device?: IpDevice
}>()
const { t } = useI18n()
const deviceTitle = computed(() => {
  const device = props.device
  if (!device) return ''
  return [
    device.name,
    device.host_name && device.host_name !== device.name ? device.host_name : '',
    [device.os, device.online ? '' : t('admin.usage.deviceOffline')].filter(Boolean).join(' · '),
    props.ip,
  ].filter(Boolean).join('\n')
})
</script>
