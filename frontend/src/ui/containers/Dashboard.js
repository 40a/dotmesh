import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import Dashboard from '../components/Dashboard'

class DashboardContainer extends Component {
  render() {
    return (
      <Dashboard {...this.props} />
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
)(DashboardContainer)