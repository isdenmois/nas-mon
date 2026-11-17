import type { TempReading } from '@/shared/api'

export function temperatureLabel(sensor: TempReading): string {
  return sensor.label ? `${sensor.chip} · ${sensor.label}` : sensor.key
}
