<template>
  <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700" :data-kind="kind">
    <div class="mb-2 flex items-center justify-between gap-2">
      <h3 class="text-sm font-medium">{{ title }}</h3>
      <div class="flex gap-2">
        <label v-if="canAdd" class="btn btn-secondary btn-sm cursor-pointer">
          {{ t('videoStudio.addFiles') }}
          <input type="file" class="hidden" :accept="accept" :multiple="multiple" @change="onFile">
        </label>
      </div>
    </div>
    <div v-if="canAdd" class="mb-2 flex gap-2">
      <input v-model="url" type="url" class="input flex-1" :placeholder="t('videoStudio.urlPlaceholder')">
      <button type="button" class="btn btn-secondary btn-sm" @click="emitUrl">{{ t('videoStudio.addUrl') }}</button>
    </div>
    <ul class="space-y-2">
      <li v-for="asset in assets" :key="asset.id" class="flex items-center gap-2 rounded-md bg-gray-50 px-2 py-1.5 text-xs dark:bg-dark-800">
        <img v-if="asset.previewUrl" :src="asset.previewUrl" alt="" class="h-8 w-8 rounded object-cover">
        <span class="min-w-0 flex-1 truncate font-mono">{{ asset.name }}</span>
        <button v-if="showInsert" type="button" class="btn btn-secondary btn-sm" @click="$emit('insert', asset)">{{ t('videoStudio.insert') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="$emit('remove', asset.id)">{{ t('videoStudio.remove') }}</button>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { VideoStudioAsset, VideoStudioAssetKind } from '@/api/videoStudio'

withDefaults(defineProps<{
  title: string
  kind: VideoStudioAssetKind
  accept: string
  assets: VideoStudioAsset[]
  canAdd: boolean
  showInsert?: boolean
  multiple?: boolean
}>(), {
  showInsert: false,
  multiple: true
})

const emit = defineEmits<{
  files: [files: File[]]
  url: [url: string]
  remove: [id: string]
  insert: [asset: VideoStudioAsset]
}>()

const { t } = useI18n()
const url = ref('')

function onFile(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (files.length) emit('files', files)
}

function emitUrl() {
  const value = url.value.trim()
  if (!value) return
  emit('url', value)
  url.value = ''
}
</script>
