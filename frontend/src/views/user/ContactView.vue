<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('contact.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('contact.subtitle') }}</p>
      </div>

      <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>

      <div v-else-if="error" class="card flex items-center justify-between gap-4 p-6" role="alert">
        <p class="text-sm text-gray-600 dark:text-gray-300">{{ error }}</p>
        <button class="btn btn-secondary btn-sm" @click="load">{{ t('contact.retry') }}</button>
      </div>

      <div v-else-if="!hasCommunity && !supportInfo" class="card flex flex-col items-center gap-3 p-12 text-center">
        <Icon name="chat" size="xl" class="text-gray-300 dark:text-dark-500" />
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('contact.empty') }}</p>
      </div>

      <div v-else class="grid gap-6" :class="hasCommunity && supportInfo ? 'lg:grid-cols-5' : ''">
        <section v-if="hasCommunity" class="card p-6" :class="supportInfo ? 'lg:col-span-3' : ''" data-testid="contact-community">
          <div class="flex items-center gap-3">
            <div class="rounded-xl bg-primary-100 p-2.5 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
              <Icon name="users" size="md" />
            </div>
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('contact.communityTitle') }}</h2>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('contact.communityHint') }}</p>
            </div>
          </div>

          <div class="mt-6 flex flex-col items-center gap-6 sm:flex-row sm:items-start">
            <img
              v-if="imageUrl"
              :src="imageUrl"
              :alt="t('contact.qrAlt')"
              class="w-full max-w-[240px] flex-shrink-0 rounded-2xl border border-gray-100 bg-white object-contain shadow-sm dark:border-dark-700"
            />
            <div class="flex w-full min-w-0 flex-1 flex-col gap-4">
              <p v-if="page?.text" class="whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-gray-300">{{ page.text }}</p>
              <div v-if="linkUrl" class="flex flex-wrap gap-3">
                <a :href="linkUrl" target="_blank" rel="noopener noreferrer" class="btn btn-primary">
                  <Icon name="externalLink" size="sm" />
                  <span>{{ t('contact.join') }}</span>
                </a>
                <button type="button" class="btn btn-secondary" @click="copyToClipboard(linkUrl, t('contact.linkCopied'))">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('contact.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>
        </section>

        <section v-if="supportInfo" class="card p-6" :class="hasCommunity ? 'lg:col-span-2' : ''" data-testid="contact-support">
          <div class="flex items-center gap-3">
            <div class="rounded-xl bg-primary-100 p-2.5 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
              <Icon name="mail" size="md" />
            </div>
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('contact.supportTitle') }}</h2>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('contact.supportHint') }}</p>
            </div>
          </div>
          <div class="mt-6 rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
            <p class="break-all font-mono text-sm text-gray-900 dark:text-gray-100">{{ supportInfo }}</p>
          </div>
          <div class="mt-4 flex flex-wrap gap-3">
            <a v-if="supportHref" :href="supportHref" :target="supportIsEmail ? undefined : '_blank'" rel="noopener noreferrer" class="btn btn-primary">
              <Icon :name="supportIsEmail ? 'mail' : 'externalLink'" size="sm" />
              <span>{{ t(supportIsEmail ? 'contact.sendEmail' : 'contact.openLink') }}</span>
            </a>
            <button type="button" class="btn btn-secondary" @click="copyToClipboard(supportInfo, t('contact.copied'))">
              <Icon name="copy" size="sm" />
              <span>{{ t('contact.copy') }}</span>
            </button>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { getContactPage, type ContactPage } from '@/api/contact'
import { useClipboard } from '@/composables/useClipboard'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const page = ref<ContactPage | null>(null)
const loading = ref(true)
const error = ref('')

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

const imageUrl = computed(() => sanitizeUrl(page.value?.image || '', { allowDataUrl: true }))
const linkUrl = computed(() => sanitizeUrl(page.value?.link || ''))
const hasCommunity = computed(() => Boolean(imageUrl.value || linkUrl.value || page.value?.text?.trim()))
const supportInfo = computed(() => page.value?.contact_info?.trim() || '')
const supportIsEmail = computed(() => EMAIL_RE.test(supportInfo.value))
const supportHref = computed(() => {
  if (supportIsEmail.value) return `mailto:${supportInfo.value}`
  return sanitizeUrl(supportInfo.value)
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    page.value = await getContactPage()
  } catch {
    error.value = t('contact.loadError')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
