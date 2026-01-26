import React from 'react'
import * as Yup from 'yup'
import { ErrorFor, ShowError } from '../utils/validation_helper'
import classNames from 'classnames'
import { withFormik, Field } from 'formik'
import { FaClipboardCheck } from 'react-icons/fa'
import { IconContext } from 'react-icons'
import i18n from 'utils/i18n'

export const Calibrate = ({ values, errors, touched, label, submitForm, complete, readOnly }) => {
  const handleSubmit = (event) => {
    event.preventDefault()
    submitForm()
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className='form-group row'>
        <label htmlFor='value' className='col-4 col-form-label'>
          {label}
        </label>

        <div className='col-4'>
          <div className='form-group'>
            <Field
              name='value'
              type='number'
              step='any'
              disabled={readOnly}
              className={classNames('form-control', {
                'is-invalid': ShowError('value', touched, errors)
              })}
            />
            <ErrorFor errors={errors} touched={touched} name='value' />
          </div>
        </div>

        <div className='col-4'>
          {complete
            ? (
              <IconContext.Provider value={{ color: 'blue', className: 'align-bottom' }}>
                <FaClipboardCheck />
              </IconContext.Provider>
              )
            : (
              <input
                type='submit'
                disabled={readOnly}
                value={i18n.t('ph:run_calibration') || 'Copy payload'}
                className='btn btn-sm btn-outline-primary'
              />
              )}
        </div>
      </div>
    </form>
  )
}

const CalibrateSchema = Yup.object().shape({
  value: Yup.number()
    .required(i18n.t('validation:number_required'))
    .typeError(i18n.t('validation:number_required'))
})

const CalibrateForm = withFormik({
  displayName: 'CalibrateForm',
  mapPropsToValues: props => {
    return {
      value: props.defaultValue
    }
  },
  validationSchema: CalibrateSchema,

  // IMPORTANT: no API call here.
  // We just call props.onSubmit(point, expectedFloat) so the wizard can copy JSON.
  handleSubmit: (values, { props }) => {
    props.onSubmit(props.point, parseFloat(values.value))
  }
})(Calibrate)

export default CalibrateForm
