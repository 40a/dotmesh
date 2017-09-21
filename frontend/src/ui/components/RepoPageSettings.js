import React, { Component, PropTypes } from 'react'

import * as selectors from '../selectors'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

class RepoPageSettings extends Component {

  render() {
    const repo = this.props.repo || {}
    return (
      <div>
        <div>
          Settings
        </div>
      </div>
    )
  }
}

export default RepoPageSettings