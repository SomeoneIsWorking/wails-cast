import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import { logger } from './utils/logger'

const app = createApp(App)

// Redirect console methods to Go backend
const originalLog = console.log
const originalWarn = console.warn
const originalError = console.error

console.log = (...args: unknown[]) => {
  const message = args.map(arg => 
    typeof arg === 'string' ? arg : JSON.stringify(arg)
  ).join(' ')
  logger.info(message)
  originalLog(...args)
}

console.warn = (...args: unknown[]) => {
  const message = args.map(arg => 
    typeof arg === 'string' ? arg : JSON.stringify(arg)
  ).join(' ')
  logger.warn(message)
  originalWarn(...args)
}

console.error = (...args: unknown[]) => {
  const message = args.map(arg => 
    typeof arg === 'string' ? arg : JSON.stringify(arg)
  ).join(' ')
  logger.error(message)
  originalError(...args)
}

// Global Vue error handler
app.config.errorHandler = (err, instance, info) => {
  const errorMessage = err instanceof Error ? err.message : String(err)
  logger.error(`[App Error] ${info}`, errorMessage)
  originalError(`[App Error] ${info}:`, err)
}

// Global warning handler
app.config.warnHandler = (msg, instance, trace) => {
  logger.warn(`[App Warning]`, msg)
  originalWarn(`[App Warning]:`, msg, trace)
}

// Unhandled promise rejection
window.addEventListener('unhandledrejection', (event) => {
  logger.error('[Unhandled Promise Rejection]', event.reason)
  originalError('[Unhandled Promise Rejection]:', event.reason)
  event.preventDefault()
})

app.use(createPinia())
app.mount('#app')
