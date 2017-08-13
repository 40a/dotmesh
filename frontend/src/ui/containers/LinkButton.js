import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import { Button } from 'react-toolbox/lib/button'

import * as actions from '../actions'
import tools from '../tools'

class LinkButton extends Component {
  render() {
    return (
      <Button
        {...this.props}
        onClick={ () => this.props.onClick(this.props.url) }
      />
    )
  }
}

export default connect(
  (state, ownProps) => ({}),
  (dispatch) => ({
    onClick: (path) => dispatch(actions.router.redirect(path))
  })
)(LinkButton)