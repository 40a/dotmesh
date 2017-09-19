import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import RepoList from '../components/RepoList'

const API_NAME = 'volumeList'

class RepoListContainer extends Component {
  render() {
    return (
      <RepoList {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => {
    return {
      error: selectors.api.error(state, API_NAME),
      loading: selectors.api.loading(state, API_NAME),
      data: selectors.repos.pageResults(state),
      search: selectors.repos.search(state)
    }
  },
  (dispatch) => ({
    updateSearch: (search) => dispatch(actions.repos.updateSearch(search))
  })
)(RepoListContainer)