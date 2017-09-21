import React, { Component, PropTypes } from 'react'

import Dropdown from 'react-toolbox/lib/dropdown'

import * as selectors from '../selectors'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

class RepoPageData extends Component {

  branchDropDown() {
    const data = (this.props.branches || []).map(branch => ({
      value: branch.name,
      label: branch.name
    }))
    return (
      <Dropdown
        auto
        source={ data }
        value={ this.props.info.Branch }
        onChange={ this.props.changeBranch }
      />
    )
  }

  render() {
    const repo = this.props.repo || {}
    return (
      <div>
        <div className={ theme.branchContainer }>
          <div className={ theme.branchTitle }>Branch:</div>
          <div className={ theme.branchDropdown }>{ this.branchDropDown() }</div>
        </div>
      </div>
    )
  }
}

export default RepoPageData