import { LogInfo, LogWarn, LogError } from '../../wailsjs/go/main/App'

type LogLevel = 'info' | 'warn' | 'error'

class Logger {
  private async log(level: LogLevel, message: string, data?: unknown) {
    const timestamp = new Date().toISOString()
    const formattedMessage = data 
      ? `${message} ${JSON.stringify(data)}` 
      : message

    switch (level) {
      case 'info':
        await LogInfo(`[${timestamp}] ${formattedMessage}`)
        break
      case 'warn':
        await LogWarn(`[${timestamp}] ${formattedMessage}`)
        break
      case 'error':
        await LogError(`[${timestamp}] ${formattedMessage}`)
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
