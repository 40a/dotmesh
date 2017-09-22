import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import UserDropdown from '../components/UserDropdown'

class UserDropdownContainer extends Component {
  render() {
    return (
      <UserDropdown {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => {
    return {
      source: [],
      value: null
    }
  },
  (dispatch) => ({
    change: (value) => {
      
    },
    queryChange: (value) => {
      
    }
  })
)(UserDropdownContainer)