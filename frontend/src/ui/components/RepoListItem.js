import React, { Component, PropTypes } from 'react'

import * as selectors from '../selectors'
import theme from './theme/repo.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

class RepoListItem extends Component {
  render() {
    const repo = this.props.repo || []
    const serverStatuses = selectors.repo.serverStatuses(repo)
    return (
      <div className={ theme.listItem }>
        <div className={ theme.repoInfo }>
          <div>
            <div className={ theme.repoName + ' ' + colors.bluelink + ' ' + theme.link } onClick={ () => this.props.clickRepo(repo) }>
              { selectors.repo.title(repo) }
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
            <StatusChip>{ selectors.repo.branchCountTitle(repo) }</StatusChip>
          </div>
        </div>
        <div className={ theme.repoStats }>
          {
            Object.keys(serverStatuses).map((key, i) => {
              return (
                <div key={ i }>{ key } { serverStatuses[key] }</div>
              )
            })
          }
        </div>
      </div>
    )
  }
}

export default RepoListItem