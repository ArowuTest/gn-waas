import { setupWorker } from 'msw/browser'
import { handlers } from './handlers'

export const worker = setupWorker(...handlers)

// Tell MSW where the service worker script lives.
// import.meta.env.BASE_URL is set by Vite's --base flag at build time:
//   GitHub Pages build → /gn-waas/admin/
//   Production / dev   → /
// Without this, MSW defaults to /mockServiceWorker.js (root) which 404s
// when the app is served from a sub-path.
export function startWorker() {
  return worker.start({
    onUnhandledRequest: 'bypass',
    serviceWorker: {
      url: `${import.meta.env.BASE_URL}mockServiceWorker.js`,
    },
  })
}
