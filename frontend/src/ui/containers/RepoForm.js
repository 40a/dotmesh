import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import FormContainer from 'template-ui/lib/plugins/form/Container'

import forms from '../forms'
import * as selectors from '../selectors'
import * as actions from '../actions'

import RepoForm from '../components/RepoForm'

const FORM = forms.repo
const Fields = FormContainer(FORM)

class RepoFormContainer extends Component {
  render() {
    return (
      <RepoForm
        {...this.props}
        fields={ <Fields /> }
      />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    isValid: selectors.form.repo.valid(state),
    formLoading: selectors.value(state, 'repoFormLoading')
  }),
  (dispatch) => ({
    submit: () => dispatch(actions.router.hook('repoFormSubmit')),
    cancel: () => dispatch(actions.router.redirect('/repos'))
  })
)(RepoFormContainer)