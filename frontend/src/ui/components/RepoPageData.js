import React, { Component, PropTypes } from 'react'

import Dropdown from 'react-toolbox/lib/dropdown'
import ProgressBar from 'react-toolbox/lib/progress_bar'

import * as selectors from '../selectors'

import SearchBox from './widgets/SearchBox'
import Pager from './widgets/Pager'

import CommitListItem from './CommitListItem'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

class RepoPageData extends Component {

  branchDropDown() {
    const data = (this.props.branches || []).map(branch => ({
      value: branch.name,
      label: branch.name
    }))
    return (
      <Dropdown
        auto
        source={ data }
        value={ this.props.info.Branch }
        onChange={ this.props.changeBranch }
        theme={{
          inputInput: theme.branchDropdownInput
        }}
      />
    )
  }

  dataList() {
    const data = this.props.commits || []
    return (
      <div className={ theme.listContainer }>
        {
          data.map((commit, i) => {
            return (
              <CommitListItem
                key={ i }
                index={ i }
                commit={ commit }
              />
            )
          })
        }
      </div>
    )
  }

  search() {
    return (
      <SearchBox
        value={ this.props.search }
        onChange={ this.props.updateSearch }
      />
    )
  }

  pager() {
    return (
      <Pager
        count={ this.props.pageCount }
        current={ this.props.pageCurrent }
        onClick={ this.props.updatePage }
      />
    )
  }

  render() {
    const repo = this.props.repo || {}
    return (
      <div>
        <div className={ theme.branchContainer }>
          <div className={ theme.branchTitle }>Branch:</div>
          <div className={ theme.branchDropdown }>{ this.branchDropDown() }</div>
          <h2 className={ theme.commitTitle }>Commits ({ this.props.searchCount })</h2>
          {
            this.props.loaded ? (
              <div>
                <div className={ theme.commitSearchContainer }>
                  { this.search() }
                </div>
                <div>
                  { this.dataList() }
                </div>
                <div>
                  { this.pager() }
                </div>
              </div>
            ) : (
              <div>
                <ProgressBar type='circular' mode='indeterminate' multicolor />
              </div>
            )
          }
        </div>
      </div>
    )
  }
}

export default RepoPageData