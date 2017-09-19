import React, { Component, PropTypes } from 'react'

import GravatarImage from './GravatarImage'
import theme from './theme/gravatar.css'

class UserImage extends Component {
  render() {
    const user = this.props.user || {}
    return (
      <div className={ theme.container }>
        <div className={ theme.avatar }>
          <GravatarImage emailHash={ user.EmailHash } />
        </div>
        <h2>{ user.Name }</h2>
      </div>
    )
  }
}

export default UserImage