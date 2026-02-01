import React from 'react'
import { fetchPhProbes, createProbe, updateProbe, deleteProbe, readProbe, fetchProbeSnapshot, clearProbeReadings  } from 'redux/actions/phprobes'
import { connect } from 'react-redux'
import PhForm from './ph_form'
import Collapsible from '../ui_components/collapsible'
import CollapsibleList from '../ui_components/collapsible_list'
import { confirm } from 'utils/confirm'
import CalibrationWizard from './calibration_wizard'
import i18next from 'i18next'
import { SortByName } from 'utils/sort_by_name'

class Chemistry extends React.Component {
  constructor (props) {
    super(props)
    this.state = {
      addProbe: false,
      showCalibrate: false,
      currentProbe: null
    }
    this.probeList = this.probeList.bind(this)
    this.handleToggleAddProbeDiv = this.handleToggleAddProbeDiv.bind(this)
    this.handleDeleteProbe = this.handleDeleteProbe.bind(this)
    this.handleCreateProbe = this.handleCreateProbe.bind(this)
    this.handleUpdateProbe = this.handleUpdateProbe.bind(this)
    this.dismissModal = this.dismissModal.bind(this)
  }

  componentDidMount () {
    this.props.fetchPhProbes()
  }

  // IMPORTANT: never mutate probe objects coming from redux store
  toggleProbeEnabled (probe) {
    const payload = { ...probe, enable: !probe.enable }
    this.props.update(probe.id, payload)
  }

  probeList () {
    return this.props.probes
      .sort((a, b) => SortByName(a, b))
      .map(probe => {
        const handleToggleState = () => this.toggleProbeEnabled(probe)

        const actionButtons = (
          <div className='btn-group float-right' role='group' aria-label='probe actions'>
            <button
              type='button'
              name={'calibrate-probe-' + probe.id}
              disabled={probe.enable}
              title={probe.enable ? 'Ph probe must be disabled before calibration' : null}
              className='btn btn-sm btn-outline-info'
              onClick={e => this.calibrateProbe(e, probe)}
            >
              {i18next.t('ph:calibrate')}
            </button>

            <button
              type='button'
              name={'clear-probe-readings-' + probe.id}
              className='btn btn-sm btn-outline-danger'
              onClick={e => this.clearReadings(e, probe)}
            >
              Clear usage
            </button>
          </div>
        )

        return (
          <Collapsible
            key={'panel-ph-' + probe.id}
            name={'panel-ph-' + probe.id}
            item={probe}
            buttons={actionButtons}
            title={<b className='ml-2 align-middle'>{probe.name} </b>}
            onDelete={this.handleDeleteProbe}
            onToggleState={handleToggleState}
            enabled={probe.enable}
          >
            <PhForm
              onSubmit={this.handleUpdateProbe}
              key={Number(probe.id)}
              analogInputs={this.props.ais}
              probe={probe}
              macros={this.props.macros}
              equipment={this.props.equipment}
            />
          </Collapsible>
        )
      })
  }

  calibrateProbe (e, probe) {
	e.preventDefault()
	e.stopPropagation()

	// Open modal first, then fetch snapshot
	// (wizard also polls, but this makes it feel instant)
	this.setState({ currentProbe: probe, showCalibrate: true }, () => {
	this.props.fetchProbeSnapshot(probe.id)
	this.props.readProbe(probe.id)
	})
 }


  clearReadings (e, probe) {
    e.preventDefault()
    e.stopPropagation()

    const message = (
      <div>
        <p>
          Clear readings history for <b>{probe.name}</b>?
        </p>
        <p className='text-muted mb-0'>
          This will wipe the stored chart/history data. New readings will start accumulating again immediately.
        </p>
      </div>
    )

    confirm(i18next.t('confirm'), { description: message }).then(
      function () {
        return this.props.clearProbeReadings(probe.id)
      }.bind(this)
    )
  }

  dismissModal () {
    this.setState({ currentProbe: null, showCalibrate: false })
  }

  valuesToProbe (values) {
    const probe = {
      name: values.name,
      enable: values.enable,
      period: values.period,
      analog_input: values.analog_input,
      temp_sensor_id: parseInt(values.temp_sensor_id || -1, 10),
      notify: {
        enable: values.notify,
        min: parseFloat(values.minAlert),
        max: parseFloat(values.maxAlert)
      },
      chart: values.chart,
      control: (values.control === 'macro' || values.control === 'equipment'),
      is_macro: (values.control === 'macro'),
      one_shot: values.one_shot || false,
      min: parseFloat(values.lowerThreshold),
      downer_eq: values.lowerFunction,
      max: parseFloat(values.upperThreshold),
      upper_eq: values.upperFunction,
      transformer: values.transformer,
      hysteresis: parseFloat(values.hysteresis),
      chart_y_min: parseInt(values.chart_y_min),
      chart_y_max: parseInt(values.chart_y_max)
    }
    return probe
  }

  handleUpdateProbe (values) {
    const payload = this.valuesToProbe(values)
    this.props.update(values.id, payload)
  }

  handleCreateProbe (values) {
    const payload = this.valuesToProbe(values)
    this.props.create(payload)
    this.handleToggleAddProbeDiv()
  }

  handleDeleteProbe (probe) {
    const message = (
      <div>
        <p>
          {i18next.t('ph:warn_delete', { name: probe.name })}
        </p>
      </div>
    )
    confirm(i18next.t('delete'), { description: message }).then(
      function () {
        this.props.delete(probe.id)
      }.bind(this)
    )
  }

  handleToggleAddProbeDiv () {
    this.setState({
      addProbe: !this.state.addProbe
    })
  }

  render () {
    let newProbe = null
    if (this.state.addProbe) {
      newProbe = (
        <PhForm
          analogInputs={this.props.ais}
          onSubmit={this.handleCreateProbe}
          macros={this.props.macros}
          equipment={this.props.equipment}
        />
      )
    }

    let calibrationModal = null
    if (this.state.showCalibrate && this.state.currentProbe) {
      const pid = this.state.currentProbe.id
      const snap = this.props.snapshots ? this.props.snapshots[pid] : null

      calibrationModal = (
        <CalibrationWizard
          probe={this.state.currentProbe}
          currentReading={this.props.currentReading}
          snapshot={snap}
          readProbe={this.props.readProbe}
          fetchProbeSnapshot={this.props.fetchProbeSnapshot}
          confirm={this.dismissModal}
          cancel={this.dismissModal}
        />
      )
    }

    return (
      <div>
        {calibrationModal}
        <ul className='list-group list-group-flush'>
          <CollapsibleList>{this.probeList()}</CollapsibleList>
          <li className='list-group-item add-probe'>
            <div className='row'>
              <div className='col'>
                <input
                  type='button'
                  id='add_probe'
                  value={this.state.addProbe ? '-' : '+'}
                  onClick={this.handleToggleAddProbeDiv}
                  className='btn btn-outline-success'
                />
              </div>
            </div>
            {newProbe}
          </li>
        </ul>
      </div>
    )
  }
}

const mapStateToProps = state => {
  return {
    probes: state.phprobes,
    ais: state.analog_inputs,
    currentReading: state.ph_reading,
    macros: state.macros,
    equipment: state.equipment,
    snapshots: state.phprobe_snapshots || {}
  }
}

const mapDispatchToProps = dispatch => {
  return {
    fetchPhProbes: () => dispatch(fetchPhProbes()),
    create: t => dispatch(createProbe(t)),
    delete: id => dispatch(deleteProbe(id)),
    update: (id, t) => dispatch(updateProbe(id, t)),
    readProbe: id => dispatch(readProbe(id)),
    fetchProbeSnapshot: id => dispatch(fetchProbeSnapshot(id)),
    clearProbeReadings: id => dispatch(clearProbeReadings(id))
  }
}


const Ph = connect(
  mapStateToProps,
  mapDispatchToProps
)(Chemistry)

export default Ph
