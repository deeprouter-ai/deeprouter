/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import '@testing-library/jest-dom'

function installStorageShim(name: 'localStorage' | 'sessionStorage') {
  if (typeof window === 'undefined') return
  const current = window[name] as Partial<Storage> | undefined
  if (
    current?.getItem &&
    current?.setItem &&
    current?.removeItem &&
    current?.clear
  ) {
    return
  }

  const data = new Map<string, string>()
  const storage: Storage = {
    get length() {
      return data.size
    },
    clear: () => data.clear(),
    getItem: (key: string) => data.get(key) ?? null,
    key: (index: number) => Array.from(data.keys())[index] ?? null,
    removeItem: (key: string) => {
      data.delete(key)
    },
    setItem: (key: string, value: string) => {
      data.set(key, String(value))
    },
  }

  Object.defineProperty(window, name, {
    configurable: true,
    value: storage,
  })
}

installStorageShim('localStorage')
installStorageShim('sessionStorage')

// jsdom implements no scrollIntoView; components that keep the highlighted
// option in view (e.g. ComboboxInput) would throw in tests without this no-op.
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {}
}

// Nor a ResizeObserver, which the command list measures itself with. Nothing
// a test looks at depends on the measurement.
if (typeof window !== 'undefined' && !window.ResizeObserver) {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}
