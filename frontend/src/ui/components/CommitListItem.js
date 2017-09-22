import React, { Component, PropTypes } from 'react'

import * as selectors from '../selectors'
import theme from './theme/repo.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

class CommitListItem extends Component {
  render() {
    const commit = this.props.commit || {}
    return (
      <div className={ theme.listItem }>
        <div className={ theme.repoInfo }>
          <div>
            <div className={ theme.commitNumber }>
              { this.props.index + 1 }.&nbsp;
            </div>
            <div className={ theme.commitName }>
              { selectors.commit.name(commit) }
            </div>
          </div>
          <div>
            <StatusChip highlight>{ selectors.commit.author(commit) }</StatusChip>
            <StatusChip>{ selectors.commit.id(commit) }</StatusChip>
          </div>
        </div>
        <div className={ theme.repoStats }>
          { selectors.commit.dateTitle(commit) } | { selectors.commit.timeTitle(commit) }
        </div>
      </div>
    )
  }
}

export default CommitListItem