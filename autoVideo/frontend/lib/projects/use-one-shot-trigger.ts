'use client'

import { useEffect } from 'react'

/**
 * Runs handler once per trigger increment. Resets the trigger immediately so tab
 * unmount/remount (Radix Tabs) does not replay the same action.
 */
export function useOneShotTriggerEffect(
  trigger: number | undefined,
  handler: () => void | Promise<void>,
  onConsumed?: () => void,
) {
  useEffect(() => {
    if (!trigger) return
    onConsumed?.()
    void handler()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trigger])
}
