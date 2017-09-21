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
  (state, ownProps) => {
    const repo = selectors.repos.getFromUrl(state)
    const branch = selectors.repos.getBranchFromUrl(state)
    const branchName = selectors.repos.extractBranchName(state)
    const urlName = selectors.repos.extractUrlName(state)
    const branchList = selectors.repo.branchList(repo)

    console.log('-------------------------------------------');
    console.log('-------------------------------------------');
    console.dir(branchName)
    console.dir(branch)
    return {
      urlName,
      repo,
      branch,
      branchName,
      branchList
    }
  },
  (dispatch) => ({
    clickNamespace: (namespace) => dispatch(actions.router.redirect('/repos')),
    clickTab: (repo, section) => dispatch(actions.router.redirect(`/${selectors.repo.url(repo)}${section ? '/' + section : ''}`)),
    changeBranch: (branchname) => dispatch(actions.router.hook('repoOpenBranch', branchname))
  })
)(RepoPageContainer)