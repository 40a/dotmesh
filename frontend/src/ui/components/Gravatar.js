import React, { Component, PropTypes } from 'react'

import theme from './theme/gravatar.css'

class Gravatar extends Component {

  getURL() {
    return `https://www.gravatar.com/avatar/${this.props.emailHash}?s=250&d=mm`
  }

  render() {
    const data = this.props.data || []
    return (
      <div className={ theme.container }>
        <div className={ theme.avatar }>
          <img src={ this.getURL() } />
        </div>
        <h2>{ this.props.name }</h2>
      </div>
    )
  }
}

export default Gravatar