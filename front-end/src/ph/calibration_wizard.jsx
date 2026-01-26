import React from 'react'
import Modal from 'modal'
import i18next from 'i18next'

function fmtNum (v, decimals) {
  if (v === null || v === undefined || Number.isNaN(v)) return '-'
  const n = Number(v)
  if (!Number.isFinite(n)) return '-'
  const d = (typeof decimals === 'number') ? decimals : 3
  return n.toFixed(d)
}

function fmtAt (iso) {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return String(iso)
  try {
    return d.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch (e) {
    return d.toLocaleString()
  }
}

function getOr (obj, key, fallback) {
  if (!obj) return fallback
  const v = obj[key]
  return (v === undefined || v === null) ? fallback : v
}

function isArr (v) { return Array.isArray(v) }

function uniq (arr) {
  const out = []
  const seen = {}
  for (let i = 0; i < arr.length; i++) {
    const k = arr[i]
    if (!k) continue
    if (seen[k]) continue
    seen[k] = true
    out.push(k)
  }
  return out
}

function sortAlpha (keys) {
  return keys.slice().sort((a, b) => {
    const aa = String(a).toLowerCase()
    const bb = String(b).toLowerCase()
    if (aa < bb) return -1
    if (aa > bb) return 1
    return 0
  })
}

function parseMaybeJSON (v) {
  if (!v) return null
  if (typeof v === 'object') return v
  if (typeof v === 'string') {
    try { return JSON.parse(v) } catch (e) { return null }
  }
  return null
}

function sig (snap, key) {
  const s = snap && snap.signals && snap.signals[key]
  if (!s) return { now: null, avg1m: null, unit: '' }
  return {
    now: (typeof s.now === 'number') ? s.now : null,
    avg1m: (typeof s.avg_1m === 'number') ? s.avg_1m : null,
    unit: s.unit || ''
  }
}

export default class CalibrationWizard extends React.Component {
  state = {
    showDerived: false,
    showMeta: false
  }

  componentDidMount () {
    if (this.props.readProbe) this.props.readProbe(this.props.probe.id)
    if (this.props.fetchProbeSnapshot) this.props.fetchProbeSnapshot(this.props.probe.id)

    this.timer = setInterval(() => {
      if (this.props.readProbe) this.props.readProbe(this.props.probe.id)
      if (this.props.fetchProbeSnapshot) this.props.fetchProbeSnapshot(this.props.probe.id)
    }, 5000)
  }

  componentWillUnmount () {
    window.clearInterval(this.timer)
  }

  handleCancel = () => this.props.cancel()
  handleConfirm = () => this.props.confirm()

  // Decimals come from driver:
  // meta.signal_decimals: { "abs_d": 3, "value": 2, ... }
  decimalsForSignal = (key, meta) => {
    const m = (meta && meta.signal_decimals) ? meta.signal_decimals : null
    if (m && typeof m === 'object' && m !== null) {
      const dv = m[key]
      if (typeof dv === 'number') return dv
      const n = Number(dv)
      if (!Number.isNaN(n) && Number.isFinite(n)) return n
    }
    return 3
  }

  // Driver-provided display names/help/roles
  displayNameFor = (key, meta) => {
    if (!key) return ''
    const dn = meta && meta.display_names
    if (dn && typeof dn === 'object' && dn !== null) {
      const v = dn[key]
      if (typeof v === 'string' && v.trim() !== '') return v
    }
    return key
  }

  displayHelpFor = (key, meta) => {
    if (!key) return ''
    const dh = meta && meta.display_help
    if (dh && typeof dh === 'object' && dh !== null) {
      const v = dh[key]
      if (typeof v === 'string' && v.trim() !== '') return v
    }
    return ''
  }

  roleLabelFor = (roleKey, meta, fallback) => {
    const dr = meta && meta.display_roles
    if (dr && typeof dr === 'object' && dr !== null) {
      const v = dr[roleKey]
      if (typeof v === 'string' && v.trim() !== '') return v
    }
    return fallback
  }

  renderSignalRowFlex = (opts) => {
    // opts: { roleLabel, key, s, decimals, meta, showKeyHint }
    const meta = opts.meta || {}
    const key = opts.key
    const s = opts.s || { now: null, avg1m: null, unit: '' }
    const decimals = (typeof opts.decimals === 'number') ? opts.decimals : 3
    const roleLabel = opts.roleLabel || ''
    const showKeyHint = Boolean(opts.showKeyHint)

    const label = roleLabel || this.displayNameFor(key, meta)
    const help = this.displayHelpFor(key, meta)

    const W = 120 // fixed column width for values

    return (
      <div
        key={`sig-${roleLabel}-${key}`}
        style={{
          display: 'flex',
          alignItems: 'baseline',
          gap: 10,
          padding: '6px 0',
          borderBottom: '1px solid rgba(0,0,0,0.04)'
        }}
      >
        <div style={{ flex: 1, minWidth: 0 }}>
          <div
            title={help || undefined}
            style={{
              fontWeight: roleLabel ? 600 : 400,
              lineHeight: '1.2',
              wordBreak: 'break-word'
            }}
          >
            {label}
          </div>

          {showKeyHint && key && (
            <div style={{ fontSize: 12, opacity: 0.65 }}>
              <code>{key}</code>
            </div>
          )}
        </div>

        <div style={{ width: W, textAlign: 'right', whiteSpace: 'nowrap' }}>
          {fmtNum(s.now, decimals)} {s.unit}
        </div>

        <div style={{ width: W, textAlign: 'right', whiteSpace: 'nowrap' }}>
          {fmtNum(s.avg1m, decimals)} {s.unit}
        </div>
      </div>
    )
  }

  renderNowAvgHeader = () => {
    const W = 120
    return (
      <div style={{ display: 'flex', gap: 10, padding: '6px 0', opacity: 0.7 }}>
        <div style={{ flex: 1 }} />
        <div style={{ width: W, textAlign: 'right' }}>now</div>
        <div style={{ width: W, textAlign: 'right' }}>avg 1m</div>
      </div>
    )
  }

  render () {
    // Guard against stale snapshot from previous probe
    const snap0 = this.props.snapshot || null
    const snap = (snap0 && String(snap0.probe_id) === String(this.props.probe.id)) ? snap0 : null
    const hasSnap = Boolean(snap)

    const metaRaw = snap ? snap.meta : null
    const meta = parseMaybeJSON(metaRaw) || (metaRaw && typeof metaRaw === 'object' ? metaRaw : {}) || {}

    const signalsObj = (snap && snap.signals) ? snap.signals : {}

    // Keys chosen by driver (chemistry adds defaults only when missing)
    const rawKey =
      getOr(meta, 'raw_signal_key', null) ||
      getOr(meta, 'calibration_observed_key', null) ||
      null

    const primaryKey =
      getOr(meta, 'primary_signal_key', null) ||
      ((signalsObj && signalsObj.value) ? 'value' : null)

    // Secondary keys: if driver provides list, DO NOT re-sort (driver decides)
    let secondaryKeys = getOr(meta, 'secondary_signal_keys', null)
    if (isArr(secondaryKeys)) {
      secondaryKeys = uniq(secondaryKeys)
    } else {
      secondaryKeys = sortAlpha(
        Object.keys(signalsObj || {}).filter(k => k !== rawKey && k !== primaryKey)
      )
    }

    const raw = rawKey ? sig(snap, rawKey) : { now: null, avg1m: null, unit: '' }
    const primary = primaryKey ? sig(snap, primaryKey) : { now: null, avg1m: null, unit: '' }

    const hasRaw = Boolean(hasSnap && rawKey && signalsObj && signalsObj[rawKey])
    const hasPrimary = Boolean(hasSnap && primaryKey && signalsObj && signalsObj[primaryKey])

    // Role labels (driver-defined if present)
    const rolePrimaryLabel = this.roleLabelFor('primary', meta, 'Primary')
    const roleObservedLabel = this.roleLabelFor('observed', meta, 'Observed')

    // meta keys (debug view)
    const metaKeys = meta ? Object.keys(meta) : []
    metaKeys.sort((a, b) => {
      const aa = String(a).toLowerCase()
      const bb = String(b).toLowerCase()
      if (aa < bb) return -1
      if (aa > bb) return 1
      return 0
    })

    const renderMetaValue = (k, v) => {
      if (typeof v === 'object' && v !== null) {
        let pretty = ''
        try { pretty = JSON.stringify(v, null, 2) } catch (e) { pretty = String(v) }
        return (
          <details>
            <summary style={{ cursor: 'pointer' }}>view</summary>
            <pre style={{ marginTop: 6, maxHeight: 220, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
              {pretty}
            </pre>
          </details>
        )
      }

      const s = String(v)
      if (s.length > 80) {
        return (
          <details>
            <summary style={{ cursor: 'pointer' }}>{s.slice(0, 80)}…</summary>
            <pre style={{ marginTop: 6, maxHeight: 220, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
              {s}
            </pre>
          </details>
        )
      }

      const isTimeKey = String(k).toLowerCase().includes('at')
      return isTimeKey ? fmtAt(s) : s
    }

    // use driver display_names for the underlying signals (no hardcoding)
    const primarySignalName = primaryKey ? this.displayNameFor(primaryKey, meta) : 'value'
    const observedSignalName = rawKey ? this.displayNameFor(rawKey, meta) : 'observed'

    return (
      <Modal>
        <div className='modal-header'>
          <h4 className='modal-title'>
            Calibration — {this.props.probe.name}
          </h4>
        </div>

        <div className='modal-body' style={{ maxHeight: '75vh', overflowY: 'auto', padding: '12px 16px' }}>
          <div className='mb-2 text-muted'>
            Snapshot at: {fmtAt(snap && snap.at)}
            {!hasSnap && <span> (waiting for snapshot…)</span>}
          </div>

          <h6>Summary</h6>

          <div style={{ marginBottom: 10 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
              <div style={{ minWidth: 0 }}>
                <b>{rolePrimaryLabel}</b>
                <div className='text-muted' style={{ fontSize: 12, lineHeight: '14px', wordBreak: 'break-word' }}>
                  {primarySignalName} {primaryKey ? <span style={{ marginLeft: 6 }}><code>{primaryKey}</code></span> : null}
                </div>
              </div>
              <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                {hasPrimary ? `${fmtNum(primary.now, this.decimalsForSignal(primaryKey || 'value', meta))} ${primary.unit}` : '-'}
              </div>
            </div>
          </div>

          <div style={{ marginBottom: 12 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
              <div style={{ minWidth: 0 }}>
                <b>{roleObservedLabel}</b>
                <div className='text-muted' style={{ fontSize: 12, lineHeight: '14px', wordBreak: 'break-word' }}>
                  {observedSignalName} {rawKey ? <span style={{ marginLeft: 6 }}><code>{rawKey}</code></span> : null}
                </div>
              </div>
              <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                {hasRaw ? `${fmtNum(raw.now, this.decimalsForSignal(rawKey || 'observed', meta))} ${raw.unit}` : '-'}
              </div>
            </div>
          </div>

          <h6>Live readings</h6>
          {this.renderNowAvgHeader()}

          {this.renderSignalRowFlex({
            roleLabel: roleObservedLabel,
            key: rawKey || 'observed',
            s: raw,
            decimals: this.decimalsForSignal(rawKey || 'observed', meta),
            meta,
            showKeyHint: true
          })}

          {this.renderSignalRowFlex({
            roleLabel: rolePrimaryLabel,
            key: primaryKey || 'value',
            s: primary,
            decimals: this.decimalsForSignal(primaryKey || 'value', meta),
            meta,
            showKeyHint: true
          })}

          {secondaryKeys.length > 0 && (
            <>
              <div className='d-flex align-items-center justify-content-between mt-2'>
                <h6 className='mb-0'>Derived signals</h6>
                <button
                  type='button'
                  className='btn btn-sm btn-outline-secondary'
                  onClick={() => this.setState({ showDerived: !this.state.showDerived })}
                >
                  {this.state.showDerived ? 'Hide' : 'Show'}
                </button>
              </div>

              {this.state.showDerived && (
                <div className='mt-2'>
                  {this.renderNowAvgHeader()}

                  {secondaryKeys.map(k => (
                    this.renderSignalRowFlex({
                      roleLabel: '', // per-signal label from driver display_names
                      key: k,
                      s: sig(snap, k),
                      decimals: this.decimalsForSignal(k, meta),
                      meta,
                      showKeyHint: true
                    })
                  ))}
                </div>
              )}

              {!this.state.showDerived && (
                <div className='text-muted mb-2'>
                  Hidden ({secondaryKeys.length}). Click “Show” to view.
                </div>
              )}
            </>
          )}

          <div className='d-flex align-items-center justify-content-between mt-3'>
            <h6 className='mb-0'>Driver meta (debug)</h6>
            <button
              type='button'
              className='btn btn-sm btn-outline-secondary'
              onClick={() => this.setState({ showMeta: !this.state.showMeta })}
            >
              {this.state.showMeta ? 'Hide' : 'Show'}
            </button>
          </div>

          {this.state.showMeta && (
            <div className='mt-2'>
              {(!hasSnap || !meta || metaKeys.length === 0) ? (
                <div className='text-muted'>No driver meta available.</div>
              ) : (
                <div>
                  {metaKeys.map((k) => {
                    const v = meta[k]
                    return (
                      <div key={`meta-${k}`} style={{ display: 'flex', gap: 10, padding: '4px 0' }}>
                        <div style={{ width: 160, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {k}
                        </div>
                        <div style={{ flex: 1, wordBreak: 'break-word' }}>
                          {renderMetaValue(k, v)}
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )}

          {!this.state.showMeta && (
            <div className='text-muted mb-2'>
              Hidden. Click “Show” to view driver meta.
            </div>
          )}
        </div>

        <div className='modal-footer'>
          <div className='text-center'>
            <button role='abort' type='button' className='btn btn-light mr-2' onClick={this.handleCancel}>
              {i18next.t('cancel')}
            </button>
            <button role='confirm' type='button' className='btn btn-primary' onClick={this.handleConfirm}>
              {i18next.t('done')}
            </button>
          </div>
        </div>
      </Modal>
    )
  }
}
