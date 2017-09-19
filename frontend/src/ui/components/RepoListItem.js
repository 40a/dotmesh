import React, { Component, PropTypes } from 'react'

import * as selectors from '../selectors'
import theme from './theme/repolist.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

class RepoListItem extends Component {
  render() {
    const repo = this.props.repo || []
    const branchCount = selectors.repo.branchCount(repo)
    return (
      <div className={ theme.listItem }>
        <div className={ theme.repoInfo }>
          <div>
            <div className={ theme.repoName + ' ' + colors.bluelink }>
              { selectors.repo.name(repo) }
            </div>
            {
              selectors.repo.isPrivate(repo) ? (
                <StatusChip
                  highlight
                >
                  Private
                </StatusChip>
              ) : null
            }
          </div>
          <div>
            <StatusChip>{ selectors.repo.sizeTitle(repo) } used</StatusChip>
            <StatusChip>{ branchCount } branch{ branchCount == 1 ? '' : 'es' }</StatusChip>
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