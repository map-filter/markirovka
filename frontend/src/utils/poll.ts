import { onBeforeUnmount } from 'vue'

export function usePoller(task: () => Promise<boolean>, intervalMs: number) {
  let timer: ReturnType<typeof setTimeout> | null = null
  let stopped = false
  let running = false

  async function tick() {
    timer = null
    if (stopped || running) return
    running = true
    let again = false
    try {
      again = await task()
    } catch {
      again = true
    } finally {
      running = false
    }
    if (again && !stopped && !document.hidden) timer = setTimeout(tick, intervalMs)
    else if (again && !stopped) timer = setTimeout(tick, intervalMs * 3)
  }

  function ensure() {
    if (stopped || timer || running) return
    timer = setTimeout(tick, intervalMs)
  }

  function stop() {
    stopped = true
    if (timer) clearTimeout(timer)
    timer = null
  }

  onBeforeUnmount(stop)
  return { ensure, stop }
}
