import React, { Component, PropTypes } from 'react'

import FormWrapper from 'template-ui/lib/components/FormWrapper'
import Link from '../containers/Link'

import colors from './theme/colors.css'

class AuthForm extends Component {
  render() {
    return (
      <div className={this.props.id}>
        <FormWrapper
          title={ this.props.title }
          submitTitle='Submit'
          fields={ this.props.fields }
          loading={ this.props.loading }
          error={ this.props.error }
          submit={ this.props.submit }
        />
        <div style={{marginTop:'20px',paddingLeft: '10px'}}>
          <Link
            url={ '/' + this.props.link }
          >
            <span className={colors.grey}>
              <span className={colors.link}>Click here</span> for the { this.props.link } form...
            </span>
          </Link>
        </div>
      </div>
    )
  }
}

export default AuthForm