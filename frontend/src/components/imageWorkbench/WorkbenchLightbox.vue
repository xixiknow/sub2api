<template>
  <BaseDialog :show="!!record" :title="t('imageWorkbench.lightbox.title')" width="extra-wide" @close="emit('close')">
    <div v-if="record && image" class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_280px]">
      <div class="flex min-h-[320px] items-center justify-center rounded-xl bg-gray-50 p-2 dark:bg-dark-900">
        <WorkbenchImage :image="image" variant="full" :alt="record.prompt" img-class="max-h-[70vh] w-auto max-w-full rounded-lg object-contain" />
      </div>
      <div class="space-y-4 text-sm">
        <div v-if="record.images.length > 1" class="flex flex-wrap gap-2">
          <button
            v-for="(item, idx) in record.images"
            :key="idx"
            type="button"
            class="h-14 w-14 overflow-hidden rounded-lg ring-2 transition"
            :class="idx === index ? 'ring-primary-500' : 'ring-transparent hover:ring-gray-300 dark:hover:ring-dark-500'"
            :aria-label="t('imageWorkbench.lightbox.imageN', { n: idx + 1 })"
            @click="index = idx"
          >
            <WorkbenchImage :image="item" img-class="h-full w-full object-cover" />
          </button>
        </div>
        <div>
          <div class="input-label">{{ t('imageWorkbench.form.prompt') }}</div>
          <p class="whitespace-pre-wrap break-words text-gray-700 dark:text-dark-200">{{ record.prompt }}</p>
          <p v-if="image.revisedPrompt" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
            {{ t('imageWorkbench.lightbox.revisedPrompt') }}：{{ image.revisedPrompt }}
          </p>
        </div>
        <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
          <dt>{{ t('imageWorkbench.form.model') }}</dt><dd class="text-gray-700 dark:text-dark-200">{{ record.model }}</dd>
          <dt>{{ t('imageWorkbench.form.apiKey') }}</dt><dd class="text-gray-700 dark:text-dark-200">{{ record.apiKeyName }}</dd>
          <template v-if="image.width"><dt>{{ t('imageWorkbench.lightbox.resolution') }}</dt><dd class="text-gray-700 dark:text-dark-200">{{ image.width }} × {{ image.height }}</dd></template>
          <dt>{{ t('imageWorkbench.lightbox.createdAt') }}</dt><dd class="text-gray-700 dark:text-dark-200">{{ new Date(record.createdAt).toLocaleString() }}</dd>
        </dl>
        <p v-if="!image.blobId && image.url" class="rounded-lg bg-amber-50 p-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
          {{ t('imageWorkbench.image.remoteOnly') }}
        </p>
        <div class="flex flex-col gap-2">
          <button type="button" class="btn btn-primary" @click="download">{{ t('imageWorkbench.actions.download') }}</button>
          <button type="button" class="btn btn-secondary" @click="emit('use-reference', record, image)">{{ t('imageWorkbench.actions.useAsReference') }}</button>
          <button type="button" class="btn btn-secondary" @click="emit('reuse', record)">{{ t('imageWorkbench.actions.reuse') }}</button>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import WorkbenchImage from './WorkbenchImage.vue'
import { downloadWorkbenchImage, type WorkbenchImage as WorkbenchImageData, type WorkbenchRecord } from '@/utils/imageWorkbenchDb'

const props = defineProps<{ record: WorkbenchRecord | null; initialIndex?: number }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'reuse', record: WorkbenchRecord): void
  (e: 'use-reference', record: WorkbenchRecord, image: WorkbenchImageData): void
}>()

const { t } = useI18n()
const index = ref(0)
watch(
  () => [props.record?.id, props.initialIndex],
  () => {
    index.value = props.initialIndex ?? 0
  },
  { immediate: true },
)
const image = computed(() => props.record?.images[index.value] ?? props.record?.images[0])

async function download() {
  if (!props.record || !image.value) return
  await downloadWorkbenchImage(image.value, `${props.record.model}-${props.record.createdAt}-${index.value + 1}`)
}
</script>

