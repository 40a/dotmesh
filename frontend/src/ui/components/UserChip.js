import React, { Component, PropTypes } from 'react'

import Chip from 'react-toolbox/lib/chip'
import UserAvatar from './UserAvatar'

import theme from './theme/userchip.css'

class UserChip extends Component {
  render() {
    const user = this.props.user || {}
    return (
      <Chip theme={ theme }>
        <UserAvatar user={ user } />
        <span className={ theme.text }>{ user.Name } sdf sdf sdfsdf</span>
      </Chip>
    )
  }
}

export default UserChip