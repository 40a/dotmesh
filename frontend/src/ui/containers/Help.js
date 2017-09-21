import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import config from '../config'
import * as selectors from '../selectors'
import * as actions from '../actions'

import Help from '../components/Help'

class HelpContainer extends Component {
  render() {
    return (
      <Help {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => {
    const menuOptions = config.menu.help
    return {
      menuOptions,
      currentPage: selectors.help.currentPage(state),
      variables: selectors.help.variables(state)
    }
  },
  (dispatch) => ({
    onMenuClick: (id) => {
      dispatch(actions.router.redirect(`/help/${id}`))
    }
  })
)(HelpContainer)