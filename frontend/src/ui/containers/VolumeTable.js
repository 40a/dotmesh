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
  (state, ownProps) => {
    const repoList = selectors.valueSelector(state, 'volumes') || []
    const repos = repoList.map(selectors.repo)

    console.log(JSON.stringify(repos, null, 4))

    return {
      error: selectors.api.error(state, API_NAME),
      loading: selectors.api.loading(state, API_NAME),
      data: repos
    }
  },
  (dispatch) => ({
    
  })
)(VolumeTableContainer)