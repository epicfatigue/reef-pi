import EditPh from './edit_ph'
import PhSchema from './ph_schema'
import { withFormik } from 'formik'

const PhForm = withFormik({
  displayName: 'PhForm',
  mapPropsToValues: props => {
    let data = props.probe
    if (data === undefined) {
      data = {
        notify: {}
      }
    }

    // Determine control type:
    // - If backend sends control_type, use it
    // - Else fall back to legacy control + is_macro
    let controlType = ''
    if (data.control_type && data.control_type !== '') {
      controlType = data.control_type // "equipment" | "macro" | "ato"
    } else if (data.control === true) {
      controlType = (data.is_macro === true) ? 'macro' : 'equipment'
    }

    // NEW: ATO-only option (safe default)
    // Expect backend JSON: "ato_in_range_disable": bool
    // If not present, default false.
    const atoInRangeDisable =
      (controlType === 'ato')
        ? (data.ato_in_range_disable === undefined ? false : !!data.ato_in_range_disable)
        : false

    const value = {
      id: data.id || '',
      name: data.name || '',
      analog_input: data.analog_input || '',
      temp_sensor_id: (data.temp_sensor_id !== undefined ? data.temp_sensor_id : -1),
      enable: (data.enable === undefined ? true : data.enable),
      period: data.period || 60,
      one_shot: data.one_shot || false,
	  ato_in_range_disable: (data.ato_in_range_disable === true),

      notify: (data.notify && data.notify.enable) || false,
      maxAlert: (data.notify && data.notify.max) || 0,
      minAlert: (data.notify && data.notify.min) || 0,

      // NEW: control type selector
      control: controlType, // '' | 'equipment' | 'macro' | 'ato'

      lowerThreshold: data.min || 0,
      lowerFunction: data.downer_eq || '',
      upperThreshold: data.max || 0,
      upperFunction: data.upper_eq || '',

      hysteresis: data.hysteresis || 0,
      transformer: data.transformer || '',
      chart: data.chart || { ymin: 0, ymax: 100, color: '#000', unit: '' },

      // NEW: ATO-only behavior
      ato_in_range_disable: atoInRangeDisable
    }

    return value
  },
  validationSchema: PhSchema,
  handleSubmit: (values, { props }) => {
    props.onSubmit(values)
  }
})(EditPh)

export default PhForm
