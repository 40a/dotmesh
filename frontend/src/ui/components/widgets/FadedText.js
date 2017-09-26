import React, { Component, PropTypes } from 'react'

import theme from './theme/fadedtext.css'

class FadedText extends Component {
  render() {
    return (
      <div className={ theme.text }>
        { this.props.children }
      </div>
    )
  }
}

export default FadedText