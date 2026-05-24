import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import './App.css'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'
const READ_API_KEY = import.meta.env.VITE_READ_API_KEY || ''

const POLL_MS = 2000

/** @typedef {'live1m' | '15m' | '1h' | 'custom'} TimePreset */

const LINE_COLORS = [
  '#64b5f6',
  '#81c784',
  '#ffb74d',
  '#e57373',
  '#ba68c8',
  '#4dd0e1',
  '#fff176',
  '#a1887f',
]

function toDatetimeLocalValue(d) {
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** `datetime-local` value (local) → UTC ISO8601 */
function localDatetimeInputToUTCISO(value) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString()
}

function assignYAxisByScale(dynamicCols) {
  const map = new Map()
  if (!dynamicCols.length) return map
  const withSpan = dynamicCols.map((col) => {
    const min = col.min_value ?? 0
    const max = col.max_value ?? min + 1
    return { name: col.name, span: Math.abs(max - min) }
  })
  withSpan.sort((a, b) => b.span - a.span)
  const mid = Math.ceil(withSpan.length / 2)
  const leftNames = new Set(withSpan.slice(0, mid).map((x) => x.name))
  dynamicCols.forEach((col) => {
    map.set(col.name, leftNames.has(col.name) ? 'left' : 'right')
  })
  return map
}

function TelemetryTooltip({ active, payload, dynamicCols }) {
  if (!active || !payload?.length) return null
  const point = payload[0].payload
  const ts = point.timestamp
  const serial = point.serial_number ?? '—'
  return (
    <div className="telemetry-tooltip">
      <div className="telemetry-tooltip__row">
        <strong>Timestamp</strong>
        <span>{ts ? new Date(ts).toISOString() : '—'}</span>
      </div>
      <div className="telemetry-tooltip__row">
        <strong>Serial</strong>
        <span>{String(serial)}</span>
      </div>
      <div className="telemetry-tooltip__metrics">
        {dynamicCols.map((col) => {
          const v = point[col.name]
          const formatted =
            v != null && v !== '' && !Number.isNaN(Number(v))
              ? Number(v).toLocaleString(undefined, { maximumFractionDigits: 6 })
              : '—'
          return (
            <div key={col.name} className="telemetry-tooltip__metric">
              <span className="telemetry-tooltip__metric-name">{col.name}</span>
              <span>{formatted}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function buildSlidingWindowISO(preset) {
  const end = new Date()
  const start = new Date(end)
  if (preset === 'live1m') start.setMinutes(start.getMinutes() - 1)
  else if (preset === '15m') start.setMinutes(start.getMinutes() - 15)
  else if (preset === '1h') start.setHours(start.getHours() - 1)
  return { start: start.toISOString(), end: end.toISOString() }
}

function App() {
  const [schema, setSchema] = useState(null)
  const [schemaError, setSchemaError] = useState(null)
  const [metrics, setMetrics] = useState(null)
  const [metricsError, setMetricsError] = useState(null)
  const [serialFilter, setSerialFilter] = useState('__all__')
  /** @type {[TimePreset, function]} */
  const [timePreset, setTimePreset] = useState('live1m')
  const [customDraftStart, setCustomDraftStart] = useState('')
  const [customDraftEnd, setCustomDraftEnd] = useState('')
  /** Applied custom window in UTC ISO; null until Apply (or when not using custom). */
  const [customRangeUTC, setCustomRangeUTC] = useState(null)
  const [customApplyError, setCustomApplyError] = useState('')

  const headers = useMemo(
    () => ({ 'X-API-Key': READ_API_KEY }),
    []
  )

  useEffect(() => {
    fetch(`${API_URL}/schema`, { headers })
      .then((r) => {
        if (!r.ok) throw new Error(r.status === 500 ? 'server_error' : `schema ${r.status}`)
        return r.json()
      })
      .then(setSchema)
      .catch(() => setSchemaError(true))
  }, [headers])

  const fetchMetrics = useCallback(
    async (signal) => {
      const params = new URLSearchParams()
      if (serialFilter !== '__all__') params.set('device', serialFilter)

      if (timePreset === 'custom') {
        if (!customRangeUTC?.start || !customRangeUTC?.end) return
        params.set('start', customRangeUTC.start)
        params.set('end', customRangeUTC.end)
      } else {
        const { start, end } = buildSlidingWindowISO(timePreset)
        params.set('start', start)
        params.set('end', end)
      }

      const qs = params.toString()
      const url = qs ? `${API_URL}/metrics?${qs}` : `${API_URL}/metrics`
      const res = await fetch(url, { headers, signal })
      if (!res.ok) {
        if (res.status === 500) throw new Error('server_error')
        if (res.status === 400) throw new Error('bad_request')
        throw new Error(`metrics ${res.status}`)
      }
      return res.json()
    },
    [headers, serialFilter, timePreset, customRangeUTC]
  )

  useEffect(() => {
    if (!schema || schemaError) return
    if (timePreset === 'custom' && (!customRangeUTC?.start || !customRangeUTC?.end)) {
      return
    }

    const ac = new AbortController()

    const run = () => {
      fetchMetrics(ac.signal)
        .then((data) => {
          setMetrics(Array.isArray(data) ? data : [])
          setMetricsError(null)
        })
        .catch((err) => {
          if (err.name === 'AbortError') return
          setMetricsError(true)
          setMetrics([])
        })
    }

    run()

    if (timePreset === 'custom') {
      return () => ac.abort()
    }

    const id = setInterval(run, POLL_MS)
    return () => {
      clearInterval(id)
      ac.abort()
    }
  }, [schema, schemaError, timePreset, customRangeUTC, fetchMetrics])

  const dynamicCols = schema?.dynamic_columns || []

  const uniqueSerials = useMemo(() => {
    if (!metrics?.length) return []
    const set = new Set()
    metrics.forEach((row) => {
      const s = row.serial_number
      if (s != null && s !== '') set.add(String(s))
    })
    return Array.from(set).sort()
  }, [metrics])

  const chartData = useMemo(() => {
    if (!metrics) return []
    return metrics
      .map((row) => {
        const point = {
          timestamp: row.timestamp,
          serial_number: row.serial_number != null ? String(row.serial_number) : '',
        }
        dynamicCols.forEach((col) => {
          const name = col.name
          if (row[name] != null && row[name] !== '') {
            const n = Number(row[name])
            if (!Number.isNaN(n)) point[name] = n
          }
        })
        return point
      })
      .sort((a, b) => {
        const ta = a.timestamp ? new Date(a.timestamp).getTime() : 0
        const tb = b.timestamp ? new Date(b.timestamp).getTime() : 0
        return ta - tb
      })
  }, [metrics, dynamicCols])

  const yAxisByMetric = useMemo(() => assignYAxisByScale(dynamicCols), [dynamicCols])
  const hasLeft = useMemo(
    () => dynamicCols.some((c) => yAxisByMetric.get(c.name) === 'left'),
    [dynamicCols, yAxisByMetric]
  )
  const hasRight = useMemo(
    () => dynamicCols.some((c) => yAxisByMetric.get(c.name) === 'right'),
    [dynamicCols, yAxisByMetric]
  )

  const handleTimePresetChange = (e) => {
    const v = /** @type {TimePreset} */ (e.target.value)
    setTimePreset(v)
    setCustomApplyError('')
    if (v === 'custom') {
      setCustomRangeUTC(null)
      const end = new Date()
      const start = new Date(end.getTime() - 60 * 60 * 1000)
      setCustomDraftStart(toDatetimeLocalValue(start))
      setCustomDraftEnd(toDatetimeLocalValue(end))
    } else {
      setCustomRangeUTC(null)
    }
  }

  const handleApplyCustom = () => {
    setCustomApplyError('')
    const startISO = localDatetimeInputToUTCISO(customDraftStart)
    const endISO = localDatetimeInputToUTCISO(customDraftEnd)
    if (!startISO || !endISO) {
      setCustomApplyError('Enter both start and end times.')
      return
    }
    if (new Date(startISO) >= new Date(endISO)) {
      setCustomApplyError('Start must be before end.')
      return
    }
    setCustomRangeUTC({ start: startISO, end: endISO })
  }

  if (schemaError) {
    return (
      <div className="app">
        <h1>Telemetry</h1>
        <p className="error-message">Unable to load metrics. Please try again.</p>
      </div>
    )
  }

  if (!schema) {
    return (
      <div className="app">
        <h1>Telemetry</h1>
        <p className="loading">Loading…</p>
      </div>
    )
  }

  const showCustomHint =
    timePreset === 'custom' && (!customRangeUTC?.start || !customRangeUTC?.end)

  return (
    <div className="app">
      <header className="dashboard-header">
        <h1>Telemetry</h1>
        <div className="dashboard-controls">
          <label className="serial-filter">
            <span className="serial-filter__label">Device</span>
            <select
              value={serialFilter}
              onChange={(e) => setSerialFilter(e.target.value)}
              className="serial-filter__select"
              aria-label="Filter by serial number"
            >
              <option value="__all__">All Devices</option>
              {uniqueSerials.map((sn) => (
                <option key={sn} value={sn}>
                  {sn}
                </option>
              ))}
            </select>
          </label>
          <label className="serial-filter">
            <span className="serial-filter__label">Time range</span>
            <select
              value={timePreset}
              onChange={handleTimePresetChange}
              className="serial-filter__select"
              aria-label="Time range preset"
            >
              <option value="live1m">Live (Last 1 Min)</option>
              <option value="15m">Last 15 Mins</option>
              <option value="1h">Last 1 Hour</option>
              <option value="custom">Custom Range</option>
            </select>
          </label>
        </div>
      </header>

      {timePreset === 'custom' && (
        <div className="custom-range-panel">
          <label className="custom-range-field">
            <span>Start time</span>
            <input
              type="datetime-local"
              value={customDraftStart}
              onChange={(e) => setCustomDraftStart(e.target.value)}
            />
          </label>
          <label className="custom-range-field">
            <span>End time</span>
            <input
              type="datetime-local"
              value={customDraftEnd}
              onChange={(e) => setCustomDraftEnd(e.target.value)}
            />
          </label>
          <button type="button" className="custom-range-apply" onClick={handleApplyCustom}>
            Apply
          </button>
          {customApplyError ? <p className="custom-range-error">{customApplyError}</p> : null}
        </div>
      )}

      {metricsError && (
        <p className="error-message">Unable to load metrics. Please try again.</p>
      )}

      {showCustomHint && !metricsError && (
        <p className="empty">Choose start and end, then click Apply to load data.</p>
      )}

      {dynamicCols.length === 0 ? (
        <p className="empty">No dynamic columns in schema.</p>
      ) : !showCustomHint ? (
        <div className="chart-card chart-card--main">
          {metrics === null ? (
            <p className="loading chart-loading">Loading chart…</p>
          ) : (
            <ResponsiveContainer width="100%" height={480}>
              <LineChart
                data={chartData}
                margin={{ top: 12, right: hasRight ? 28 : 16, left: hasLeft ? 8 : 16, bottom: 12 }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="#444" />
                <XAxis
                  dataKey="timestamp"
                  tick={{ fill: '#aaa', fontSize: 11 }}
                  tickFormatter={(v) => (v ? new Date(v).toLocaleTimeString() : '')}
                />
                {hasLeft && (
                  <YAxis
                    yAxisId="left"
                    orientation="left"
                    tick={{ fill: '#aaa', fontSize: 11 }}
                    width={56}
                  />
                )}
                {hasRight && (
                  <YAxis
                    yAxisId="right"
                    orientation="right"
                    tick={{ fill: '#aaa', fontSize: 11 }}
                    width={56}
                  />
                )}
                <Tooltip
                  content={<TelemetryTooltip dynamicCols={dynamicCols} />}
                  wrapperStyle={{ outline: 'none' }}
                />
                <Legend />
                {dynamicCols.map((col, i) => {
                  const yAxisId = yAxisByMetric.get(col.name) || 'left'
                  return (
                    <Line
                      key={col.name}
                      type="monotone"
                      dataKey={col.name}
                      name={col.name}
                      stroke={LINE_COLORS[i % LINE_COLORS.length]}
                      strokeWidth={2}
                      dot={false}
                      connectNulls
                      yAxisId={yAxisId}
                    />
                  )
                })}
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      ) : null}
    </div>
  )
}

export default App
