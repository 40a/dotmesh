import React, { Component, PropTypes } from 'react'

import * as selectors from '../selectors'
import theme from './theme/repo.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

class CollaboratorListItem extends Component {
  render() {
    const collaborator = this.props.collaborator || {}
    return (
      <div className={ theme.listItem }>
        <div className={ theme.repoInfo }>
          <div>
            <div className={ theme.commitName }>
              { selectors.user.name(collaborator) }
            </div>
          </div>
        </div>
      </div>
    )
  }
}

export default CollaboratorListItem