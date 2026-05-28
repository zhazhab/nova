const NARRATIVE_START = '<NARRATIVE>'
const NARRATIVE_END = '</NARRATIVE>'
const HOT_STATE_START = '<HOT_STATE>'
const STATE_DELTA_START = '<STATE_DELTA>'

const TAGS = [NARRATIVE_START, NARRATIVE_END, HOT_STATE_START, STATE_DELTA_START]

export function createInteractiveNarrativeFilter() {
  let buffer = ''
  let stopped = false

  return {
    push(chunk: string): string {
      if (!chunk || stopped) return ''
      buffer += chunk
      return drain(false)
    },
    flush(): string {
      if (stopped) return ''
      return drain(true)
    },
  }

  function drain(flushAll: boolean): string {
    let output = ''
    while (buffer) {
      if (buffer.startsWith(HOT_STATE_START) || buffer.startsWith(STATE_DELTA_START)) {
        stopped = true
        buffer = ''
        return output
      }
      if (buffer.startsWith(NARRATIVE_START)) {
        buffer = buffer.slice(NARRATIVE_START.length)
        continue
      }
      if (buffer.startsWith(NARRATIVE_END)) {
        buffer = buffer.slice(NARRATIVE_END.length)
        buffer = buffer.trimStart()
        continue
      }

      const nextTag = findNextTag(buffer)
      if (nextTag > 0) {
        output += buffer.slice(0, nextTag)
        buffer = buffer.slice(nextTag)
        continue
      }
      if (nextTag === 0) continue

      const keep = flushAll ? 0 : partialTagSuffixLength(buffer)
      output += buffer.slice(0, buffer.length - keep)
      buffer = buffer.slice(buffer.length - keep)
      return output
    }
    return output
  }
}

function findNextTag(value: string): number {
  let next = -1
  for (const tag of TAGS) {
    const index = value.indexOf(tag)
    if (index >= 0 && (next < 0 || index < next)) next = index
  }
  return next
}

function partialTagSuffixLength(value: string): number {
  const max = Math.min(value.length, Math.max(...TAGS.map((tag) => tag.length)) - 1)
  for (let length = max; length > 0; length--) {
    const suffix = value.slice(value.length - length)
    if (TAGS.some((tag) => tag.startsWith(suffix))) return length
  }
  return 0
}
