import { phProbesLoaded, probeReadingsLoaded, fetchPhProbes, fetchProbeReadings, probeUpdated, probeCalibrated, updateProbe, calibrateProbe, deleteProbe, createProbe } from './phprobes'
import thunk from 'redux-thunk'
import fetchMock from 'fetch-mock'
import configureMockStore from 'redux-mock-store'
import 'isomorphic-fetch'

const middlewares = [thunk]
const mockStore = configureMockStore(middlewares)

const CHEMISTRY_PROBES_API = '/api/chemistryprobes'

describe('probe actions', () => {
  afterEach(() => {
    fetchMock.reset()
    fetchMock.restore()
  })

  it('phProbesLoaded', () => {
    expect(phProbesLoaded({}).type).toEqual('PH_PROBES_LOADED')
  })

  it('probeReadingsLoaded', () => {
    expect(probeReadingsLoaded('1')({}).type).toEqual('PH_PROBE_READINGS_LOADED')
  })

  it('probeUpdated', () => {
    expect(probeUpdated({}).type).toEqual('PH_PROBE_UPDATED')
  })

  it('probeCalibrated', () => {
    expect(probeCalibrated({}).type).toEqual('PH_PROBE_CALIBRATED')
  })

  it('fetchPhProbes', () => {
    fetchMock.getOnce(CHEMISTRY_PROBES_API, {})
    const store = mockStore()
    return store.dispatch(fetchPhProbes()).then(() => {
      expect(store.getActions()).toEqual([phProbesLoaded({})])
    })
  })

  it('fetchProbeReadings', () => {
    fetchMock.getOnce(CHEMISTRY_PROBES_API + '/1/readings', {})
    const store = mockStore()
    return store.dispatch(fetchProbeReadings('1')).then(() => {
      expect(store.getActions()).toEqual([probeReadingsLoaded('1')({})])
    })
  })

  it('createProbe', () => {
    fetchMock.putOnce(CHEMISTRY_PROBES_API, {})
    fetchMock.getOnce(CHEMISTRY_PROBES_API, {})
    const store = mockStore()
    return store.dispatch(createProbe({})).then(() => {
      expect(store.getActions()).toEqual([phProbesLoaded({})])
    })
  })

  it('updateProbe', () => {
    fetchMock.postOnce(CHEMISTRY_PROBES_API + '/1', {})
    fetchMock.getOnce(CHEMISTRY_PROBES_API, {})
    const store = mockStore()
    return store.dispatch(updateProbe('1')).then(() => {
      expect(store.getActions()).toEqual([phProbesLoaded({})])
    })
  })

  it('calibrateProbe', () => {
    fetchMock.postOnce(CHEMISTRY_PROBES_API + '/1/calibratepoint', {})
    const store = mockStore()
    return store.dispatch(calibrateProbe('1')).then(() => {
      expect(store.getActions()).toEqual([probeCalibrated({})])
    })
  })

  it('deleteProbe', () => {
    fetchMock.deleteOnce(CHEMISTRY_PROBES_API + '/1', {})
    fetchMock.getOnce(CHEMISTRY_PROBES_API, {})
    const store = mockStore()
    return store.dispatch(deleteProbe('1')).then(() => {
      expect(store.getActions()).toEqual([phProbesLoaded({})])
    })
  })
})
