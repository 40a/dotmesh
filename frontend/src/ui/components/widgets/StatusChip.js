import React, { Component, PropTypes } from 'react'

import theme from './theme/statuschip.css'
/*

  this needs some kind of border & background
  
*/
class StatusChip extends Component {
  render() {
    let classNames = [theme.chip]
    if(this.props.highlight) {
      classNames.push(theme.highlight)
    }
    return (
      <div className={ classNames.join(' ') }>{ this.props.children }</div>
    )
  }
}

export default StatusChip