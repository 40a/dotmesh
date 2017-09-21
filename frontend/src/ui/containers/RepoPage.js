import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import RepoPage from '../components/RepoPage'

class RepoPageContainer extends Component {
  render() {
    return (
      <RepoPage {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    repo: selectors.repos.getFromUrl(state)
  }),
  (dispatch) => ({
    clickNamespace: (namespace) => dispatch(actions.router.redirect('/repos'))
  })
)(RepoPageContainer)