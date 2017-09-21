import React, { Component, PropTypes } from 'react'

import FormWrapper from 'template-ui/lib/components/FormWrapper'
import Link from '../containers/Link'

import colors from './theme/colors.css'

class RepoForm extends Component {

  getActions() {
    return [{
      label: 'Cancel',
      onClick: this.props.cancel
    },{
      label: 'Submit',
      raised: true,
      primary: true,
      onClick: this.props.submit
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