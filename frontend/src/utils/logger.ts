import { LogInfo, LogWarn, LogError } from '../../wailsjs/go/main/App'

type LogLevel = 'info' | 'warn' | 'error'

class Logger {
  private async log(level: LogLevel, message: string, data?: unknown) {
    const timestamp = new Date().toISOString()
    const fullMessage = data 
      ? `[${timestamp}] ${message} ${JSON.stringify(data)}`
      : `[${timestamp}] ${message}`

    switch (level) {
      case 'info':
        await LogInfo(fullMessage, [])
        break
      case 'warn':
        await LogWarn(fullMessage, [])
        break
      case 'error':
        await LogError(fullMessage, [])
        break
    }
  }

  info(message: string, data?: unknown) {
    this.log('info', message, data)
  }

  warn(message: string, data?: unknown) {
    this.log('warn', message, data)
  }

  error(message: string, data?: unknown) {
    this.log('error', message, data)
  }
}

export const logger = new Logger()
