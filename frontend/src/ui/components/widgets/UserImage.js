import React, { Component, PropTypes } from 'react'

import GravatarImage from './GravatarImage'
import theme from './theme/gravatar.css'

class UserImage extends Component {
  render() {
    const user = this.props.user || {}
    return (
      <div className={ theme.container }>
        <div className={ theme.avatar }>
          <GravatarImage
            emailHash={ user.EmailHash }
            size={ this.props.size }
            className={ this.props.imageClassName }
          />
          <div className={ theme.username }>
            <h2>{ user.Name }</h2>
          </div>
        </div>
      </div>
    )
  }
}

export default UserImage