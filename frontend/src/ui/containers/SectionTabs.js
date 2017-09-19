import React, { Component, PropTypes } from 'react'
import { connect } from 'react-redux'

import * as selectors from '../selectors'
import * as actions from '../actions'

import SectionTabs from '../components/SectionTabs'

class SectionTabsContainer extends Component {
  render() {
    return (
      <SectionTabs {...this.props} />
    )
  }
}

export default connect(
  (state, ownProps) => ({
    repoCount: selectors.repos.count(state),
    searchCount: selectors.repos.searchCount(state)
  }),
  (dispatch) => ({
    link: (path) => dispatch(actions.router.redirect(path))
  })
)(SectionTabsContainer)