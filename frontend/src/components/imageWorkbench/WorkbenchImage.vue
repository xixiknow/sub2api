<template>
  <img
    v-if="src"
    :src="src"
    :alt="alt"
    :class="imgClass"
    loading="lazy"
    decoding="async"
    @error="failed = true"
  />
  <div v-else :class="[imgClass, 'flex items-center justify-center bg-gray-100 text-xs text-gray-400 dark:bg-dark-800 dark:text-dark-500']">
    {{ failed ? t('imageWorkbench.image.unavailable') : '' }}
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getBlob, type WorkbenchImage } from '@/utils/imageWorkbenchDb'

const props = withDefaults(
  defineProps<{
    image: WorkbenchImage
    /** thumb：优先缩略图；full：原图 */
    variant?: 'thumb' | 'full'
    alt?: string
    imgClass?: string
  }>(),
  { variant: 'thumb', alt: '', imgClass: '' },
)

const { t } = useI18n()
const src = ref('')
const failed = ref(false)
let objectUrl = ''

function release() {
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  objectUrl = ''
}

async function load() {
  release()
  failed.value = false
  src.value = ''
  const blobId = props.variant === 'thumb' ? props.image.thumbId || props.image.blobId : props.image.blobId
  if (blobId) {
    const blob = await getBlob(blobId).catch(() => undefined)
    if (blob) {
      objectUrl = URL.createObjectURL(blob)
      src.value = objectUrl
      return
    }
  }
  if (props.image.url) {
    src.value = props.image.url
    return
  }
  failed.value = true
}

watch(() => [props.image.blobId, props.image.thumbId, props.image.url, props.variant], load, { immediate: true })
onBeforeUnmount(release)
</script>
