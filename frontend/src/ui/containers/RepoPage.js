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

    const info = selectors.repoPage.urlInfo(state)
    const repo = selectors.repos.getRepo(state, info)
    
    const branches = repo ? selectors.repo.branchList(repo) : []
    const branch = selectors.repo.getBranch(repo, info.Branch)

    const commits = selectors.commits.all(state)
    const section = selectors.router.firstValue(state, 'repoPageSection')

    return {
      loaded: selectors.value(state, 'repoPageDataLoaded'),
      repo,
      branches,
      branch,
      info,
      section,
      commits: selectors.commits.pageResults(state),
      search: selectors.commits.search(state),
      commitCount: selectors.commits.count(state),
      searchCount: selectors.commits.searchCount(state),
      pageCount: selectors.commits.pageCount(state),
      pageCurrent: selectors.commits.pageCurrent(state)
    }
  },
  (dispatch) => ({
    updateSearch: (search) => dispatch(actions.router.hook('repoCommitUpdateSearch', search)),
    updatePage: (page) => dispatch(actions.router.hook('repoCommitUpdatePage', page)),
    clickNamespace: (namespace) => dispatch(actions.router.redirect('/repos')),
    clickTab: (repo, section) => dispatch(actions.router.redirect(`/${selectors.repo.url(repo)}${section ? '/' + section : ''}`)),
    changeBranch: (branchname) => dispatch(actions.router.hook('repoOpenBranch', branchname))
  })
)(RepoPageContainer)