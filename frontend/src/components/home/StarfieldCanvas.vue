<template>
  <canvas ref="canvasRef" class="pointer-events-none block h-full w-full" aria-hidden="true"></canvas>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

/**
 * 全屏星空背景：三层视差、鼠标星座连线、流星、跃迁（warp）。
 * - warp: 0 = 静止星空，0~1 = 星星沿径向拉伸成光线的强度
 * - zoom: 0~1，随页面滚动放大星空，营造「驶入星海」的纵深
 */
const props = withDefaults(
  defineProps<{
    warp?: number
    zoom?: number
    reducedMotion?: boolean
    links?: boolean
    /** 亮色主题：星星改用深色墨点，关闭流星 */
    light?: boolean
  }>(),
  { warp: 0, zoom: 0, reducedMotion: false, links: true, light: false }
)

interface Star {
  x: number
  y: number
  z: number
  r: number
  phase: number
  speed: number
  color: number
  alpha: number
}

interface Meteor {
  x: number
  y: number
  vx: number
  vy: number
  life: number
  max: number
}

const STAR_COLORS = ['255,255,255', '255,255,255', '255,240,214', '199,210,254', '253,230,138', '165,243,252']
// 亮色主题下与 STAR_COLORS 一一对应的深色版本
const STAR_COLORS_LIGHT = ['30,27,75', '49,46,129', '146,64,14', '67,56,202', '180,83,9', '14,116,144']
const LINK_RADIUS = 170
const LINK_DIST = 120

const canvasRef = ref<HTMLCanvasElement | null>(null)

let ctx: CanvasRenderingContext2D | null = null
let width = 0
let height = 0
let stars: Star[] = []
let meteors: Meteor[] = []
let raf = 0
let running = false
let nextMeteorAt = 0
let warpEased = 0
let zoomEased = 0
let lastNow = 0
const mouse = { x: -9999, y: -9999, nx: 0, ny: 0, ex: 0, ey: 0, active: false }
const finePointer = typeof window !== 'undefined' && window.matchMedia('(pointer: fine)').matches

function rand(min: number, max: number) {
  return Math.random() * (max - min) + min
}

function makeStar(fadeIn = false): Star {
  const z = Math.random() < 0.6 ? 1 : Math.random() < 0.75 ? 2 : 3
  return {
    x: rand(-width / 2, width / 2),
    y: rand(-height / 2, height / 2),
    z,
    r: z === 1 ? rand(0.35, 0.9) : z === 2 ? rand(0.8, 1.4) : rand(1.3, 2.1),
    phase: rand(0, Math.PI * 2),
    speed: rand(0.6, 2.2),
    color: Math.floor(Math.random() * STAR_COLORS.length),
    alpha: fadeIn ? 0 : 1
  }
}

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  width = canvas.clientWidth
  height = canvas.clientHeight
  canvas.width = Math.floor(width * dpr)
  canvas.height = Math.floor(height * dpr)
  ctx = canvas.getContext('2d')
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0)
  const area = width * height
  const count = Math.min(finePointer ? 520 : 220, Math.floor(area / (finePointer ? 2600 : 4200)))
  stars = Array.from({ length: count }, () => makeStar())
  if (!running) draw(performance.now())
}

function spawnMeteor(now: number) {
  const fromLeft = Math.random() < 0.5
  const speed = rand(9, 14)
  const angle = rand(0.35, 0.6)
  meteors.push({
    x: fromLeft ? rand(-width * 0.1, width * 0.5) : rand(width * 0.5, width * 1.1),
    y: rand(-40, height * 0.35),
    vx: (fromLeft ? 1 : -1) * Math.cos(angle) * speed,
    vy: Math.sin(angle) * speed,
    life: 0,
    max: rand(50, 80)
  })
  nextMeteorAt = now + rand(2600, 6500)
}

function draw(now: number) {
  if (!ctx) return
  const c = ctx
  const t = now / 1000
  const cx = width / 2
  const cy = height / 2

  // 以 60fps 为基准的帧时间系数，保证低帧率（后台/节流）下动画速度一致
  const k = lastNow ? Math.min(4, (now - lastNow) / 16.67) : 1
  lastNow = now
  warpEased += (props.warp - warpEased) * Math.min(1, 0.06 * k)
  zoomEased += (props.zoom - zoomEased) * Math.min(1, 0.08 * k)
  mouse.ex += (mouse.nx - mouse.ex) * Math.min(1, 0.05 * k)
  mouse.ey += (mouse.ny - mouse.ey) * Math.min(1, 0.05 * k)

  c.clearRect(0, 0, width, height)
  const scale = 1 + zoomEased * 0.55
  const maxR = Math.hypot(cx, cy)
  const near: { x: number; y: number }[] = []

  for (const s of stars) {
    if (s.alpha < 1) s.alpha = Math.min(1, s.alpha + 0.02 * k)

    // 跃迁：沿径向向外飞，越远越快
    let streak = 0
    if (warpEased > 0.01 && !props.reducedMotion) {
      const d = Math.hypot(s.x, s.y) || 1
      const v = (0.4 + s.z * 0.9) * warpEased * (4 + (d / maxR) * 26)
      s.x += (s.x / d) * v * k
      s.y += (s.y / d) * v * k
      streak = v * 3.2
      if (Math.abs(s.x) * scale > cx + 40 || Math.abs(s.y) * scale > cy + 40) {
        Object.assign(s, makeStar(true), {
          x: rand(-width * 0.25, width * 0.25),
          y: rand(-height * 0.25, height * 0.25)
        })
        continue
      }
    }

    const px = cx + s.x * scale - mouse.ex * s.z * 14
    const py = cy + s.y * scale - mouse.ey * s.z * 14
    if (px < -20 || px > width + 20 || py < -20 || py > height + 20) continue

    const twinkle = props.reducedMotion ? 0.8 : 0.55 + 0.45 * Math.sin(t * s.speed + s.phase)
    const a = twinkle * s.alpha * (s.z === 1 ? 0.6 : 0.9) * (props.light ? 0.45 : 1)
    const rgb = (props.light ? STAR_COLORS_LIGHT : STAR_COLORS)[s.color]

    if (streak > 1) {
      const d = Math.hypot(s.x, s.y) || 1
      c.strokeStyle = `rgba(${rgb},${Math.min(1, a + 0.2)})`
      c.lineWidth = s.r
      c.beginPath()
      c.moveTo(px, py)
      c.lineTo(px - (s.x / d) * streak, py - (s.y / d) * streak)
      c.stroke()
    } else {
      c.fillStyle = `rgba(${rgb},${a})`
      c.beginPath()
      c.arc(px, py, s.r, 0, Math.PI * 2)
      c.fill()
      if (s.z === 3) {
        c.fillStyle = `rgba(${rgb},${a * 0.12})`
        c.beginPath()
        c.arc(px, py, s.r * 4, 0, Math.PI * 2)
        c.fill()
      }
    }

    if (props.links && mouse.active && s.z > 1 && near.length < 18) {
      const dx = px - mouse.x
      const dy = py - mouse.y
      if (dx * dx + dy * dy < LINK_RADIUS * LINK_RADIUS) near.push({ x: px, y: py })
    }
  }

  // 鼠标星座连线
  if (near.length > 1 && warpEased < 0.2) {
    c.lineWidth = 0.7
    for (let i = 0; i < near.length; i++) {
      for (let j = i + 1; j < near.length; j++) {
        const d = Math.hypot(near[i].x - near[j].x, near[i].y - near[j].y)
        if (d < LINK_DIST) {
          c.strokeStyle = `rgba(196,181,253,${(1 - d / LINK_DIST) * 0.55})`
          c.beginPath()
          c.moveTo(near[i].x, near[i].y)
          c.lineTo(near[j].x, near[j].y)
          c.stroke()
        }
      }
      const dm = Math.hypot(near[i].x - mouse.x, near[i].y - mouse.y)
      c.strokeStyle = `rgba(253,230,138,${(1 - dm / LINK_RADIUS) * 0.25})`
      c.beginPath()
      c.moveTo(near[i].x, near[i].y)
      c.lineTo(mouse.x, mouse.y)
      c.stroke()
    }
  }

  // 流星
  if (!props.reducedMotion && !props.light) {
    if (now > nextMeteorAt && warpEased < 0.1) spawnMeteor(now)
    meteors = meteors.filter((m) => m.life < m.max)
    for (const m of meteors) {
      m.life += k
      m.x += m.vx * k
      m.y += m.vy * k
      const fade = Math.sin((m.life / m.max) * Math.PI)
      const tail = 11
      const grad = c.createLinearGradient(m.x, m.y, m.x - m.vx * tail, m.y - m.vy * tail)
      grad.addColorStop(0, `rgba(255,255,255,${0.9 * fade})`)
      grad.addColorStop(0.3, `rgba(253,224,171,${0.45 * fade})`)
      grad.addColorStop(1, 'rgba(253,224,171,0)')
      c.strokeStyle = grad
      c.lineWidth = 1.6
      c.beginPath()
      c.moveTo(m.x, m.y)
      c.lineTo(m.x - m.vx * tail, m.y - m.vy * tail)
      c.stroke()
    }
  }
}

function loop(now: number) {
  draw(now)
  raf = requestAnimationFrame(loop)
}

function start() {
  if (running || props.reducedMotion) return
  running = true
  lastNow = 0
  nextMeteorAt = performance.now() + 1500
  raf = requestAnimationFrame(loop)
}

function stop() {
  running = false
  cancelAnimationFrame(raf)
}

function onPointerMove(e: PointerEvent) {
  if (!finePointer) return
  mouse.x = e.clientX
  mouse.y = e.clientY
  mouse.nx = (e.clientX / window.innerWidth) * 2 - 1
  mouse.ny = (e.clientY / window.innerHeight) * 2 - 1
  mouse.active = true
}

function onPointerLeave() {
  mouse.active = false
  mouse.x = mouse.y = -9999
}

function onVisibility() {
  if (document.hidden) stop()
  else start()
}

watch(
  () => props.light,
  () => {
    if (!running) draw(performance.now())
  }
)

watch(
  () => props.reducedMotion,
  (reduced) => {
    if (reduced) {
      stop()
      draw(performance.now())
    } else {
      start()
    }
  }
)

onMounted(() => {
  resize()
  window.addEventListener('resize', resize)
  window.addEventListener('pointermove', onPointerMove, { passive: true })
  document.documentElement.addEventListener('pointerleave', onPointerLeave)
  document.addEventListener('visibilitychange', onVisibility)
  start()
})

onBeforeUnmount(() => {
  stop()
  window.removeEventListener('resize', resize)
  window.removeEventListener('pointermove', onPointerMove)
  document.documentElement.removeEventListener('pointerleave', onPointerLeave)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>
