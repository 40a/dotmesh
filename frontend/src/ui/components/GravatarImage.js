import React, { Component, PropTypes } from 'react'

import theme from './theme/gravatar.css'

class GravatarImage extends Component {

  getURL() {
    const size = this.props.size || 250
    const emailHash = this.props.emailHash
    return `https://www.gravatar.com/avatar/${emailHash}?s=${size}&d=mm`
  }

  render() {
    return (
      <img src={ this.getURL() } />
    )
  }
}

export default GravatarImage