const isDevelopment = import.meta.env.MODE === 'development';

export const logger = {
  log: (...args: any[]) => {
    if (isDevelopment) {
      console.log(`[LOG][${new Date().toISOString()}]`, ...args);
    }
  },
  warn: (...args: any[]) => {
    if (isDevelopment) {
      console.warn(`[WARN][${new Date().toISOString()}]`, ...args);
    }
  },
  error: (...args: any[]) => {
    if (isDevelopment) {
      console.error(`[ERROR][${new Date().toISOString()}]`, ...args);
    }
  },
};