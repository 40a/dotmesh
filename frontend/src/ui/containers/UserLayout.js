import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import UserLayout from '../components/UserLayout'

class UserLayoutContainer extends Component {
  render() {
    return (
      <UserLayout {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => {
    return {
      user: selectors.auth.user(state)
    }
  },
  (dispatch) => ({
    
  })
)(UserLayoutContainer)