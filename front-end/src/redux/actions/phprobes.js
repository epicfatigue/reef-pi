import { reduxPut, reduxDelete, reduxGet, reduxPost } from '../../utils/ajax'

// New canonical base path (legacy /api/phprobes remains as an alias server-side)
const CHEMISTRY_PROBES_API = '/api/chemistryprobes'

// Snapshot (driver truth)
const PROBE_SNAPSHOT_LOADED = 'PROBE_SNAPSHOT_LOADED'

export const probeSnapshotLoaded = (payload) => ({
  type: PROBE_SNAPSHOT_LOADED,
  payload
})

export const fetchProbeSnapshot = (id) => {
  return (reduxDispatch) => {
    return fetch(`${CHEMISTRY_PROBES_API}/${id}/snapshot`, {
      method: 'GET',
      credentials: 'same-origin'
    })
      .then((response) => response.json())
      .then((data) => reduxDispatch(probeSnapshotLoaded(data)))
  }
}

export const phProbesLoaded = (s) => {
  return ({
    type: 'PH_PROBES_LOADED',
    payload: s
  })
}

export const probeReadingsLoaded = (id) => {
  return (s) => {
    return ({
      type: 'PH_PROBE_READINGS_LOADED',
      payload: { readings: s, id: id }
    })
  }
}

export const probeUpdated = () => {
  return ({
    type: 'PH_PROBE_UPDATED'
  })
}

export const probeCalibrated = () => {
  return ({
    type: 'PH_PROBE_CALIBRATED'
  })
}

export const fetchPhProbes = () => {
  return (reduxGet({
    url: CHEMISTRY_PROBES_API,
    success: phProbesLoaded
  }))
}

export const readProbe = (id) => {
  return (reduxGet({
    url: CHEMISTRY_PROBES_API + '/' + id + '/read',
    success: probeReadComplete(id)
  }))
}

export const probeReadComplete = (id) => {
  return (s) => {
    return ({
      type: 'PH_PROBE_READING_COMPLETE',
      payload: { reading: s, id: id }
    })
  }
}

export const fetchProbeReadings = (id) => {
  return (reduxGet({
    url: CHEMISTRY_PROBES_API + '/' + id + '/readings',
    success: probeReadingsLoaded(id)
  }))
}

export const clearProbeReadings = (id) => {
  return (reduxPost({
    url: CHEMISTRY_PROBES_API + '/' + id + '/readings/clear',
    success: () => fetchProbeReadings(id)
  }))
}


export const updateProbe = (id, a) => {
  return (reduxPost({
    url: CHEMISTRY_PROBES_API + '/' + id,
    data: a,
    success: fetchPhProbes
  }))
}

export const createProbe = (a) => {
  return (reduxPut({
    url: CHEMISTRY_PROBES_API,
    data: a,
    success: fetchPhProbes
  }))
}

export const deleteProbe = (id) => {
  return (reduxDelete({
    url: CHEMISTRY_PROBES_API + '/' + id,
    success: fetchPhProbes
  }))
}

export const calibrateProbe = (id, a) => {
  return (reduxPost({
    url: CHEMISTRY_PROBES_API + '/' + id + '/calibratepoint',
    data: a,
    success: probeCalibrated
  }))
}
