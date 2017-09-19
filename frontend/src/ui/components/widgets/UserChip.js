import React, { Component, PropTypes } from 'react'

import Chip from 'react-toolbox/lib/chip'
import UserAvatar from './UserAvatar'

import theme from './theme/userchip.css'

class UserChip extends Component {
  render() {
    const user = this.props.user || {}
    return (
      <Chip theme={ theme }>
        <span className={ theme.text }>{ user.Name }</span>
        <UserAvatar user={ user } />
      </Chip>
    )
  }
}

export default UserChip