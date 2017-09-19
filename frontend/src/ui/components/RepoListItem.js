import React, { Component, PropTypes } from 'react'

import * as selectors from '../selectors'
import theme from './theme/repolist.css'
import colors from './theme/colors.css'

class RepoListItem extends Component {
  render() {
    const repo = this.props.repo || []
    return (
      <div className={ theme.listItem }>
        <div className={ theme.repoInfo }>
          <div className={ theme.repoName + ' ' + colors.bluelink }>
            { selectors.repo.name(repo) }
          </div>
        </div>
        <div className={ theme.repoStats }>
          stats
        </div>
      </div>
    )
  }
}

export default RepoListItem