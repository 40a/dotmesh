import React, { Component, PropTypes } from 'react'
import Navigation from 'react-toolbox/lib/navigation'

import config from '../../config'
import theme from './theme/pager.css'

// control the page size in config.repolist.pageSize

class Pager extends Component {

  buttons() {
    let buttons = []

    buttons.push({
      icon: config.icons.back,
      onClick: () => this.props.onClick(this.props.current-1),
      disabled: this.props.current<=1
    })
    for(let i=1; i<=this.props.count; i++) {      
      buttons.push({
        label: '' + i,
        primary: i == this.props.current,
        onClick: () => this.props.onClick(i)
      })
    }
    buttons.push({
      icon: config.icons.forward,
      onClick: () => this.props.onClick(this.props.current+1),
      disabled: this.props.current==this.props.count
    })
    return buttons
  }

  render() {
    return (
      <div className={ theme.container }>
        <div className={ theme.nagivation }>
          <Navigation 
            theme={ theme }
            type='horizontal'
            actions={ this.buttons() }
          />
        </div>
      </div>
    )
  }
}

export default Pager