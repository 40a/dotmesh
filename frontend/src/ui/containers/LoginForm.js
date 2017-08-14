import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import FormContainer from 'template-ui/lib/plugins/form/Container'
import FormWrapper from 'template-ui/lib/components/FormWrapper'

import Link from './Link'
import forms from '../forms'

import * as selectors from '../selectors'
import * as actions from '../actions'

import colors from '../components/theme/colors.css'

const FORM = forms.authLogin

const Fields = FormContainer(FORM)

class LoginForm extends Component {
  render() {
    return (
      <div>
        <FormWrapper
          title='Login'
          submitTitle='Submit'
          fields={ <Fields /> }
          loading={ this.props.loading }
          error={ this.props.error }
          submit={ this.props.submit }
        />
        <div style={{marginTop:'20px',paddingLeft: '10px'}}>
          <Link
            url='/register'
          >
            <span className={colors.pink}>Click here for the register form...</span>
          </Link>
        </div>
      </div>
    )
  }
}

export default connect(
  (state, ownProps) => ({
    error: selectors.api.error(state, FORM.name),
    loading: selectors.api.loading(state, FORM.name)
  }),
  (dispatch) => ({
    submit: () => dispatch(actions.router.hook('authLoginSubmit'))
  })
)(LoginForm)