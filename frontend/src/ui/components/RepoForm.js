import React, { Component, PropTypes } from 'react'

import FormWrapper from 'template-ui/lib/components/FormWrapper'
import Link from '../containers/Link'

import colors from './theme/colors.css'

class RepoForm extends Component {

  getActions() {
    return [{
      label: 'Cancel',
      onClick: this.props.cancel,
      disabled: this.props.formLoading
    },{
      label: 'Submit',
      raised: this.props.isValid,
      primary: this.props.isValid,
      onClick: this.props.submit,
      disabled: this.props.formLoading
    }]
  }

  render() {
    return (
      <div>
        <FormWrapper
          title={ this.props.title }
          submitTitle='Submit'
          fields={ this.props.fields }
          loading={ this.props.loading }
          error={ this.props.error }
          actions={ this.getActions() }
        />
      </div>
    )
  }
}

export default RepoForm