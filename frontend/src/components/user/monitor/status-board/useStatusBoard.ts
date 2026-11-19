import { computed, ref, type Ref } from 'vue'
import {
  BOARD_PLATFORM_TABS,
  BOARD_SORT_DEFAULT_DIR,
  type BoardPlatformFilter,
  type BoardPlatformTab,
  type BoardSort,
  type BoardSortKey,
  type BoardStatus,
  type StatusCardModel,
} from './types'

/** Platform aliases that should land on a named tab instead of "other". */
const PLATFORM_ALIASES: Record<string, BoardPlatformTab> = {
  xai: 'grok',
  claude: 'anthropic',
}

export function boardPlatformTab(platform: string | null | undefined): BoardPlatformTab {
  const key = String(platform || '').toLowerCase()
  if ((BOARD_PLATFORM_TABS as readonly string[]).includes(key) && key !== 'other') {
    return key as BoardPlatformTab
  }
  return PLATFORM_ALIASES[key] ?? 'other'
}

export function countBoardPlatforms(cards: StatusCardModel[]): Record<BoardPlatformFilter, number> {
  const counts = { all: cards.length } as Record<BoardPlatformFilter, number>
  for (const tab of BOARD_PLATFORM_TABS) counts[tab] = 0
  for (const card of cards) counts[boardPlatformTab(card.platform)] += 1
  return counts
}

const STATUS_RANK: Record<BoardStatus, number> = {
  operational: 0,
  degraded: 1,
  failed: 2,
  unknown: 3,
}

function sortValue(card: StatusCardModel, key: BoardSortKey): number | null {
  switch (key) {
    case 'status':
      return STATUS_RANK[card.status]
    case 'rate':
    case 'success':
    case 'cache':
    case 'latency': {
      const value = card.sort[key]
      return value == null || Number.isNaN(value) ? null : value
    }
    default:
      return null
  }
}

/**
 * Filter by platform tab + search keyword, then sort. Cards without a value for
 * the active sort key always go last; ties keep the incoming (default) order.
 */
export function filterAndSortBoard(
  cards: StatusCardModel[],
  platform: BoardPlatformFilter,
  search: string,
  sort: BoardSort,
): StatusCardModel[] {
  const keyword = search.trim().toLowerCase()
  const filtered = cards.filter((card) => {
    if (platform !== 'all' && boardPlatformTab(card.platform) !== platform) return false
    if (keyword && !card.searchText.includes(keyword)) return false
    return true
  })
  if (sort.key === 'default') return filtered
  const direction = sort.dir === 'asc' ? 1 : -1
  return filtered
    .map((card, index) => ({ card, index, value: sortValue(card, sort.key) }))
    .sort((a, b) => {
      if (a.value == null && b.value == null) return a.index - b.index
      if (a.value == null) return 1
      if (b.value == null) return -1
      if (a.value !== b.value) return (a.value - b.value) * direction
      return a.index - b.index
    })
    .map((entry) => entry.card)
}

/** Clicking the active key flips direction; a new key starts at its natural direction. */
export function nextBoardSort(current: BoardSort, key: BoardSortKey): BoardSort {
  if (key === 'default') return { key, dir: 'asc' }
  if (current.key === key) return { key, dir: current.dir === 'asc' ? 'desc' : 'asc' }
  return { key, dir: BOARD_SORT_DEFAULT_DIR[key] }
}

export function useStatusBoard(cards: Ref<StatusCardModel[]>) {
  const platform = ref<BoardPlatformFilter>('all')
  const search = ref('')
  const sort = ref<BoardSort>({ key: 'default', dir: 'asc' })

  const counts = computed(() => countBoardPlatforms(cards.value))
  const visibleCards = computed(() =>
    filterAndSortBoard(cards.value, platform.value, search.value, sort.value),
  )

  return { platform, search, sort, counts, visibleCards }
}
