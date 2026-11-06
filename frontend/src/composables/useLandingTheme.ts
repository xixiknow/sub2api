import { ref } from 'vue'

/**
 * 落地页 / 认证页的亮暗主题。
 * - 访客手动选过主题（localStorage 'theme'）时沿用该选择，与全站一致；
 * - 没选过时默认深色（星空主题在深色下效果最好），不跟随系统。
 * 切换时同步写入 localStorage 与 <html> 的 dark 类，进入控制台后保持一致。
 */
const landingDark = ref(true)

function readLandingTheme(): boolean {
  try {
    const saved = localStorage.getItem('theme')
    return saved ? saved === 'dark' : true
  } catch {
    return true
  }
}

export function useLandingTheme() {
  landingDark.value = readLandingTheme()

  function toggleLandingTheme() {
    landingDark.value = !landingDark.value
    try {
      localStorage.setItem('theme', landingDark.value ? 'dark' : 'light')
    } catch {
      // 隐私模式等无法写入时仅本次生效
    }
    document.documentElement.classList.toggle('dark', landingDark.value)
  }

  return { landingDark, toggleLandingTheme }
}
