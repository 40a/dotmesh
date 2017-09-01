import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import ServerTable from '../components/ServerTable'

const API_NAME = 'volumeList'

class ServerTableContainer extends Component {
  render() {
    return (
      <ServerTable {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    error: selectors.api.error(state, API_NAME),
    loading: selectors.api.loading(state, API_NAME),
    data: selectors.valueSelector(state, 'servers')
  }),
  (dispatch) => ({
    
  })
)(ServerTableContainer)