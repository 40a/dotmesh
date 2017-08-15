import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import FormContainer from 'template-ui/lib/plugins/form/Container'

import forms from '../forms'
import * as selectors from '../selectors'
import * as actions from '../actions'

import AuthForm from '../components/AuthForm'

const FORM = forms.authRegister
const Fields = FormContainer(FORM)

class RegisterFormContainer extends Component {
  render() {
    return (
      <AuthForm
        {...this.props}
        id='RegisterForm'
        title='Register'
        link='login'
        fields={ <Fields /> }
      />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    error: selectors.api.error(state, FORM.name),
    loading: selectors.api.loading(state, FORM.name)
  }),
  (dispatch) => ({
    submit: () => dispatch(actions.router.hook('authRegisterSubmit'))
  })
)(RegisterFormContainer)