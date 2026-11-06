<template>
  <canvas ref="canvasRef" class="pointer-events-none fixed inset-0 z-[5] block h-full w-full" aria-hidden="true"></canvas>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

/**
 * 光尘粒子场：数千颗白金色光尘沿图形轮廓排列。
 * - 页面中带 data-particle-section="<shape>" 的区块进入视口中线时，光尘流动重组成该区块的图形；
 *   图形绘制在区块内 [data-particle-anchor] 元素的位置（随页面滚动），没有锚点时散成满屏星尘。
 * - 鼠标划过会把光尘推开，轮廓像弹性薄膜一样凹陷，移开后缓慢回弹。
 * - burst 递增时光尘向四周炸开（配合跃迁）。
 */
const props = withDefaults(defineProps<{ reducedMotion?: boolean; burst?: number; light?: boolean }>(), {
  reducedMotion: false,
  burst: 0,
  light: false
})

type ShapeName = 'st' | 'orbit' | 'bars' | 'cursor' | 'star' | 'scatter'

interface ShapePoint {
  x: number
  y: number
  group: number
}

interface ShapeDef {
  w: number
  h: number
  points: ShapePoint[]
  /** 动态分组（如轨道上的行星）：返回该组在 t 时刻的中心偏移 */
  dynamic?: (group: number, t: number) => { x: number; y: number } | null
  /** 分组透明度（如光标闪烁） */
  groupAlpha?: (group: number, t: number) => number
}

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  k: number
  swirl: number
  delay: number
  size: number
  color: number
  base: number
  phase: number
  fade: number
}

const COLORS = ['#ffffff', '#fde68a', '#c4b5fd']
// 亮色主题：深靛蓝为主，夹杂琥珀与紫色
const COLORS_LIGHT = ['#312e81', '#b45309', '#7c3aed']
const HALO = 9
const canvasRef = ref<HTMLCanvasElement | null>(null)

let ctx: CanvasRenderingContext2D | null = null
let vw = 0
let vh = 0
let particles: Particle[] = []
let sections: { el: HTMLElement; shape: ShapeName; anchor: HTMLElement | null }[] = []
let activeIndex = -1
let activeDef: ShapeDef | null = null
let raf = 0
let lastNow = 0
const cache = new Map<string, ShapeDef>()
const mouse = { x: -9999, y: -9999, active: false }

function rand(a: number, b: number) {
  return a + Math.random() * (b - a)
}

// ==================== 图形生成 ====================

/** 在离屏画布上绘制实心图形，取其边缘像素作为轮廓点 */
function edgePoints(w: number, h: number, draw: (c: CanvasRenderingContext2D) => void): { x: number; y: number }[] {
  const cv = document.createElement('canvas')
  cv.width = Math.max(1, Math.round(w))
  cv.height = Math.max(1, Math.round(h))
  const c = cv.getContext('2d', { willReadFrequently: true })
  if (!c) return []
  c.fillStyle = '#fff'
  c.strokeStyle = '#fff'
  draw(c)
  const W = cv.width
  const H = cv.height
  const data = c.getImageData(0, 0, W, H).data
  const on = (x: number, y: number) => x >= 0 && y >= 0 && x < W && y < H && data[(y * W + x) * 4 + 3] > 127
  const pts: { x: number; y: number }[] = []
  const step = 2
  for (let y = 0; y < H; y += step) {
    for (let x = 0; x < W; x += step) {
      if (!on(x, y)) continue
      if (!on(x - step, y) || !on(x + step, y) || !on(x, y - step) || !on(x, y + step)) pts.push({ x, y })
    }
  }
  return pts
}

function sample(src: { x: number; y: number }[], n: number, group: number, jitter = 2.6): ShapePoint[] {
  const out: ShapePoint[] = []
  if (!src.length) return out
  for (let i = 0; i < n; i++) {
    const p = src[Math.floor(Math.random() * src.length)]
    // 两个均匀分布相加≈三角分布：中心密、边缘疏，形成有厚度的光带
    out.push({ x: p.x + (rand(-jitter, jitter) + rand(-jitter, jitter)) / 1.4, y: p.y + (rand(-jitter, jitter) + rand(-jitter, jitter)) / 1.4, group })
  }
  return out
}

function halo(w: number, h: number, n: number): ShapePoint[] {
  return Array.from({ length: n }, () => {
    const a = Math.random() * Math.PI * 2
    const r = Math.sqrt(Math.random())
    return { x: w / 2 + Math.cos(a) * r * w * 0.55, y: h / 2 + Math.sin(a) * r * h * 0.55, group: HALO }
  })
}

function sparklePath(c: CanvasRenderingContext2D, cx: number, cy: number, r: number) {
  const k = r * 0.16
  c.beginPath()
  c.moveTo(cx, cy - r)
  c.bezierCurveTo(cx + k, cy - k, cx + k, cy - k, cx + r, cy)
  c.bezierCurveTo(cx + k, cy + k, cx + k, cy + k, cx, cy + r)
  c.bezierCurveTo(cx - k, cy + k, cx - k, cy + k, cx - r, cy)
  c.bezierCurveTo(cx - k, cy - k, cx - k, cy - k, cx, cy - r)
  c.closePath()
}

/** 把各分组按权重分配到 n 个点，并按 y 排序，使相邻图形间的光尘大致自上而下对应流动 */
function compose(n: number, w: number, h: number, parts: { pts: { x: number; y: number }[]; weight: number; group: number; jitter?: number }[], haloRatio = 0.05): ShapePoint[] {
  const haloN = Math.round(n * haloRatio)
  const rest = n - haloN
  const total = parts.reduce((s, p) => s + (p.pts.length ? p.weight : 0), 0) || 1
  let points: ShapePoint[] = []
  parts.forEach((p, i) => {
    const count = i === parts.length - 1 ? rest - points.length : Math.round((rest * p.weight) / total)
    points = points.concat(sample(p.pts, Math.max(0, count), p.group, p.jitter))
  })
  points = points.concat(halo(w, h, n - points.length))
  return points.map((p) => ({ p, key: p.y + rand(-30, 30) })).sort((a, b) => a.key - b.key).map((x) => x.p)
}

function buildShape(name: ShapeName, w: number, h: number, n: number): ShapeDef {
  const m = Math.min(w, h)

  if (name === 'st') {
    const text = edgePoints(w, h, (c) => {
      let fs = h * 0.8
      c.font = `italic 900 ${fs}px "Arial Black", "Helvetica Neue", Arial, sans-serif`
      const tw = c.measureText('ST').width
      fs = Math.min(fs, (fs * w * 0.8) / tw)
      c.font = `italic 900 ${fs}px "Arial Black", "Helvetica Neue", Arial, sans-serif`
      c.textBaseline = 'alphabetic'
      c.fillText('ST', w * 0.03, h * 0.5 + fs * 0.36)
    })
    const star = edgePoints(w, h, (c) => {
      sparklePath(c, w * 0.88, h * 0.16, m * 0.13)
      c.fill()
    })
    return {
      w,
      h,
      points: compose(n, w, h, [
        { pts: text, weight: 0.82, group: 0 },
        { pts: star, weight: 0.18, group: 1 }
      ])
    }
  }

  if (name === 'orbit') {
    const cx = w / 2
    const cy = h / 2
    const rings = [
      { rx: w * 0.42, ry: h * 0.17, rot: -0.42 },
      { rx: w * 0.4, ry: h * 0.15, rot: 0.5 },
      { rx: w * 0.18, ry: h * 0.44, rot: 0.08 }
    ]
    const ringPts = edgePoints(w, h, (c) => {
      c.lineWidth = Math.max(5, m * 0.016)
      rings.forEach((r) => {
        c.beginPath()
        c.ellipse(cx, cy, r.rx, r.ry, r.rot, 0, Math.PI * 2)
        c.stroke()
      })
    })
    const core = edgePoints(w, h, (c) => {
      c.beginPath()
      c.arc(cx, cy, m * 0.06, 0, Math.PI * 2)
      c.fill()
    })
    const planetR = m * 0.035
    const planet = edgePoints(planetR * 2 + 4, planetR * 2 + 4, (c) => {
      c.beginPath()
      c.arc(planetR + 2, planetR + 2, planetR, 0, Math.PI * 2)
      c.fill()
    }).map((p) => ({ x: p.x - planetR - 2, y: p.y - planetR - 2 }))
    const speeds = [0.55, -0.42, 0.33]
    return {
      w,
      h,
      points: compose(n, w, h, [
        { pts: ringPts, weight: 0.76, group: 0 },
        { pts: core, weight: 0.1, group: 1 },
        { pts: planet, weight: 0.045, group: 2, jitter: 1 },
        { pts: planet, weight: 0.045, group: 3, jitter: 1 },
        { pts: planet, weight: 0.045, group: 4, jitter: 1 }
      ]),
      dynamic: (group, t) => {
        if (group < 2 || group > 4) return null
        const r = rings[group - 2]
        const a = t * speeds[group - 2] + group * 2.1
        const ex = r.rx * Math.cos(a)
        const ey = r.ry * Math.sin(a)
        return { x: cx + ex * Math.cos(r.rot) - ey * Math.sin(r.rot), y: cy + ex * Math.sin(r.rot) + ey * Math.cos(r.rot) }
      }
    }
  }

  if (name === 'bars') {
    const heights = [0.36, 0.62, 0.48, 0.9]
    const bw = w * 0.14
    const gap = (w * 0.86 - bw * 4) / 3
    const bottom = h * 0.94
    const bars = edgePoints(w, h, (c) => {
      heights.forEach((ht, i) => {
        const x = w * 0.07 + i * (bw + gap)
        const top = bottom - ht * h * 0.86
        c.beginPath()
        c.roundRect(x, top, bw, bottom - top, 6)
        c.fill()
      })
    })
    const trend = edgePoints(w, h, (c) => {
      c.lineWidth = Math.max(4, m * 0.012)
      c.lineCap = 'round'
      c.beginPath()
      heights.forEach((ht, i) => {
        const x = w * 0.07 + i * (bw + gap) + bw / 2
        const y = bottom - ht * h * 0.86 - h * 0.06
        if (i === 0) c.moveTo(x, y)
        else c.lineTo(x, y)
      })
      c.stroke()
    })
    return {
      w,
      h,
      points: compose(n, w, h, [
        { pts: bars, weight: 0.84, group: 0 },
        { pts: trend, weight: 0.16, group: 1 }
      ])
    }
  }

  if (name === 'cursor') {
    const chevron = edgePoints(w, h, (c) => {
      c.lineWidth = m * 0.1
      c.lineJoin = 'miter'
      c.lineCap = 'square'
      c.beginPath()
      c.moveTo(w * 0.14, h * 0.22)
      c.lineTo(w * 0.44, h * 0.5)
      c.lineTo(w * 0.14, h * 0.78)
      c.stroke()
    })
    const underscore = edgePoints(w, h, (c) => {
      c.fillRect(w * 0.54, h * 0.7, w * 0.32, m * 0.09)
    })
    return {
      w,
      h,
      points: compose(n, w, h, [
        { pts: chevron, weight: 0.66, group: 0 },
        { pts: underscore, weight: 0.34, group: 1 }
      ]),
      groupAlpha: (group, t) => (group === 1 ? (Math.floor(t * 1.25) % 2 === 0 ? 1 : 0.08) : 1)
    }
  }

  if (name === 'star') {
    const star = edgePoints(w, h, (c) => {
      sparklePath(c, w / 2, h / 2, m * 0.4)
      c.fill()
    })
    const ring = edgePoints(w, h, (c) => {
      c.lineWidth = Math.max(4, m * 0.012)
      c.beginPath()
      c.ellipse(w / 2, h / 2, m * 0.48, m * 0.16, -0.35, 0, Math.PI * 2)
      c.stroke()
    })
    return {
      w,
      h,
      points: compose(n, w, h, [
        { pts: star, weight: 0.7, group: 0 },
        { pts: ring, weight: 0.3, group: 1 }
      ])
    }
  }

  // scatter：满屏星尘
  return {
    w,
    h,
    points: Array.from({ length: n }, () => ({ x: Math.random() * w, y: Math.random() * h, group: HALO }))
  }
}

function getShape(name: ShapeName, w: number, h: number): ShapeDef {
  const key = `${name}-${Math.round(w)}x${Math.round(h)}`
  let def = cache.get(key)
  if (!def) {
    def = buildShape(name, w, h, particles.length)
    cache.set(key, def)
  }
  return def
}

// ==================== 运行时 ====================

function anchorRect(i: number) {
  const s = sections[i]
  if (!s || !s.anchor || s.shape === 'scatter') return { left: 0, top: 0, width: vw, height: vh }
  const r = s.anchor.getBoundingClientRect()
  return { left: r.left, top: r.top, width: r.width, height: r.height }
}

function activate(i: number) {
  activeIndex = i
  const s = sections[i]
  const r = anchorRect(i)
  activeDef = getShape(s ? s.shape : 'scatter', r.width, r.height)
  for (const p of particles) p.delay = rand(0, 28)
}

function pickSection() {
  const mid = vh * 0.5
  let best = activeIndex
  let bestDist = Infinity
  sections.forEach((s, i) => {
    const r = s.el.getBoundingClientRect()
    if (r.top <= mid && r.bottom >= mid) {
      best = i
      bestDist = 0
    } else if (bestDist > 0) {
      const d = Math.min(Math.abs(r.top - mid), Math.abs(r.bottom - mid))
      if (d < bestDist) {
        bestDist = d
        best = i
      }
    }
  })
  if (best !== activeIndex && best >= 0) activate(best)
}

function step(now: number) {
  if (!ctx) return
  const c = ctx
  const t = now / 1000
  const f = lastNow ? Math.min(3, (now - lastNow) / 16.67) : 1
  lastNow = now

  pickSection()
  const rect = anchorRect(activeIndex)
  if (activeDef && (Math.abs(activeDef.w - rect.width) > 4 || Math.abs(activeDef.h - rect.height) > 4)) {
    activeDef = getShape(sections[activeIndex]?.shape || 'scatter', rect.width, rect.height)
  }
  const def = activeDef
  if (!def) return
  const isScatter = !sections[activeIndex] || sections[activeIndex].shape === 'scatter'
  const dyn = new Map<number, { x: number; y: number } | null>()
  const R = vw < 768 ? 70 : 110
  const damping = Math.pow(0.9, f)

  c.clearRect(0, 0, vw, vh)

  // 图形背后的柔光
  if (!isScatter && rect.top + rect.height > -200 && rect.top < vh + 200) {
    const cx = rect.left + rect.width / 2
    const cy = rect.top + rect.height / 2
    const g = c.createRadialGradient(cx, cy, 0, cx, cy, Math.max(rect.width, rect.height) * 0.62)
    g.addColorStop(0, props.light ? 'rgba(167,139,250,0.14)' : 'rgba(253,230,138,0.07)')
    g.addColorStop(0.5, props.light ? 'rgba(251,191,36,0.06)' : 'rgba(217,70,239,0.04)')
    g.addColorStop(1, 'rgba(99,102,241,0)')
    c.fillStyle = g
    c.fillRect(cx - rect.width, cy - rect.height, rect.width * 2, rect.height * 2)
  }

  let lastColor = -1
  const palette = props.light ? COLORS_LIGHT : COLORS
  // 深色背景叠加发光；亮色背景叠加会发灰，改为普通绘制
  c.globalCompositeOperation = props.light ? 'source-over' : 'lighter'
  for (let i = 0; i < particles.length; i++) {
    const p = particles[i]
    const pt = def.points[i]
    if (!pt) continue
    let ox = 0
    let oy = 0
    if (def.dynamic && pt.group >= 2) {
      if (!dyn.has(pt.group)) dyn.set(pt.group, def.dynamic(pt.group, t))
      const d = dyn.get(pt.group)
      if (d) {
        ox = d.x
        oy = d.y
      }
    }
    const amp = isScatter ? 14 : pt.group === HALO ? 6 : 0.9
    const tx = rect.left + pt.x + ox + Math.sin(t * 0.7 + p.phase) * amp
    const ty = rect.top + pt.y + oy + Math.cos(t * 0.6 + p.phase * 1.3) * amp

    if (props.reducedMotion) {
      p.x = tx
      p.y = ty
    } else {
      const dx = tx - p.x
      const dy = ty - p.y
      let strength = 1
      if (p.delay > 0) {
        p.delay -= f
        strength = 0.12
      }
      let ax = dx * p.k * strength - dy * p.swirl * strength
      let ay = dy * p.k * strength + dx * p.swirl * strength
      if (mouse.active) {
        const mx = p.x - mouse.x
        const my = p.y - mouse.y
        const d2 = mx * mx + my * my
        if (d2 < R * R && d2 > 0.01) {
          const d = Math.sqrt(d2)
          const push = (1 - d / R) * (1 - d / R) * 3.2
          ax += (mx / d) * push
          ay += (my / d) * push
        }
      }
      p.vx = (p.vx + ax * f) * damping
      p.vy = (p.vy + ay * f) * damping
      p.x += p.vx * f
      p.y += p.vy * f
    }

    if (p.y < -10 || p.y > vh + 10 || p.x < -10 || p.x > vw + 10) continue
    if (p.fade < 1) p.fade = Math.min(1, p.fade + 0.015 * f)
    const speed = Math.abs(p.vx) + Math.abs(p.vy)
    const groupA = def.groupAlpha ? def.groupAlpha(pt.group, t) : 1
    const baseA = pt.group === HALO ? (isScatter ? 0.3 : 0.18) : p.base * (props.light ? 1 : 0.85)
    const a = Math.min(1, baseA * groupA + Math.min(0.45, speed * 0.04)) * p.fade
    if (a < 0.02) continue
    if (p.color !== lastColor) {
      c.fillStyle = palette[p.color]
      lastColor = p.color
    }
    c.globalAlpha = a
    c.fillRect(p.x, p.y, p.size, p.size)
  }
  c.globalAlpha = 1
  c.globalCompositeOperation = 'source-over'
}

function loop(now: number) {
  step(now)
  raf = requestAnimationFrame(loop)
}

function start() {
  if (raf || document.hidden) return
  lastNow = 0
  raf = requestAnimationFrame(loop)
}

function stop() {
  cancelAnimationFrame(raf)
  raf = 0
}

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  vw = window.innerWidth
  vh = window.innerHeight
  canvas.width = Math.floor(vw * dpr)
  canvas.height = Math.floor(vh * dpr)
  ctx = canvas.getContext('2d')
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0)
  cache.clear()
  if (activeIndex >= 0) activate(activeIndex)
}

function createParticles() {
  const n = window.innerWidth < 768 ? 1800 : 5200
  particles = Array.from({ length: n }, (_, i) => {
    const r = Math.random()
    return {
      x: Math.random() * window.innerWidth,
      y: Math.random() * window.innerHeight,
      vx: 0,
      vy: 0,
      k: rand(0.007, 0.013),
      swirl: rand(-0.0025, 0.0025),
      delay: 0,
      size: r < 0.75 ? rand(0.8, 1.3) : rand(1.3, 1.9),
      color: i % 10 < 6 ? 0 : i % 10 < 9 ? 1 : 2,
      base: rand(0.55, 1),
      phase: rand(0, Math.PI * 2),
      fade: 0
    }
  })
}

function collectSections() {
  sections = Array.from(document.querySelectorAll<HTMLElement>('[data-particle-section]')).map((el) => ({
    el,
    shape: (el.dataset.particleSection as ShapeName) || 'scatter',
    anchor: el.querySelector<HTMLElement>('[data-particle-anchor]')
  }))
}

function onPointerMove(e: PointerEvent) {
  mouse.x = e.clientX
  mouse.y = e.clientY
  mouse.active = true
}

function onPointerLeave() {
  mouse.active = false
}

function onVisibility() {
  if (document.hidden) stop()
  else start()
}

watch(
  () => props.burst,
  () => {
    const cx = vw / 2
    const cy = vh / 2
    for (const p of particles) {
      const dx = p.x - cx
      const dy = p.y - cy
      const d = Math.hypot(dx, dy) || 1
      const s = rand(10, 26)
      p.vx += (dx / d) * s
      p.vy += (dy / d) * s
      p.delay = 60
    }
  }
)

onMounted(async () => {
  await nextTick()
  createParticles()
  resize()
  collectSections()
  pickSection()
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
