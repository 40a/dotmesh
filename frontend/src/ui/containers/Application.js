import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import config from '../config'
import * as actions from '../actions'
import * as selectors from '../selectors'

import Application from '../components/Application'

class ApplicationContainer extends Component {
  render() {
    return (
      <Application {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => {
    const user = selectors.auth.user(state)
    const menuOptions = user ?
      config.menu.user :
      config.menu.guest
    const autoScroll = selectors.router.firstValue(state, 'autoScroll')
    return {
      title: config.title,
      menuOpen: selectors.valueSelector(state, 'menuOpen'),
      user,
      menuOptions,
      autoScroll: autoScroll,
      initialized: selectors.valueSelector(state, 'initialized'),
      message: selectors.application.message(state)
    }
  },
  (dispatch) => {
    return {
      toggleMenu: () => dispatch(actions.value.toggle('menuOpen')),
      onMenuClick: (id) => {
        dispatch(actions.value.set('menuOpen', false))
        dispatch(actions.router.redirect(id))
      },
      onOptionClick: (id) => {
        dispatch(actions.router.redirect(id))
      },
      clearMessage: () => dispatch(actions.application.clearMessage())
    }
  }
)(ApplicationContainer)