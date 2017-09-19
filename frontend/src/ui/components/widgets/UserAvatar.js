import React, { Component, PropTypes } from 'react'

import Avatar from 'react-toolbox/lib/avatar'
import GravatarImage from './GravatarImage'

class UserAvatar extends Component {
  render() {
    return (
      <Avatar>
        <GravatarImage size={64} emailHash={ this.props.user.EmailHash } />
      </Avatar>
    )
  }
}

export default UserAvatar