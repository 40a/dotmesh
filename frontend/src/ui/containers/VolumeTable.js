import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import VolumeTable from '../components/VolumeTable'

const API_NAME = 'volumeList'

class VolumeTableContainer extends Component {
  render() {
    return (
      <VolumeTable {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    error: selectors.api.error(state, API_NAME),
    loading: selectors.api.loading(state, API_NAME),
    data: selectors.valueSelector(state, 'volumes')
  }),
  (dispatch) => ({
    
  })
)(VolumeTableContainer)