import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import { Button } from 'react-toolbox/lib/button'

import * as actions from '../actions'
import tools from '../tools'

class Link extends Component {
  render() {
    return (
      <div
        style={{cursor:'pointer'}}
        onClick={ () => this.props.onClick(this.props.url) }
      >
        { this.props.children }
      </div>
    )
  }
}

export default connect(
  (state, ownProps) => ({}),
  (dispatch) => ({
    onClick: (path) => dispatch(actions.router.redirect(path))
  })
)(Link)