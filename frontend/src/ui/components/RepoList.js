import React, { Component, PropTypes } from 'react'

import Navigation from 'react-toolbox/lib/navigation'

import config from '../config'
import RepoListItem from './RepoListItem'
import Pager from './widgets/Pager'
import SearchBox from './widgets/SearchBox'
import theme from './theme/repolist.css'

class RepoList extends Component {

  noData() {
    return (
      <div className={ theme.container }>
        No repos - display help page
      </div>
    )
  }

  dataList() {
    const data = this.props.data || []
    return (
      <div className={ theme.listContainer }>
        {
          data.map((repo, i) => {
            return (
              <RepoListItem
                key={ i }
                repo={ repo }
              />
            )
          })
        }
      </div>
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

  optionsBar() {
    return (
      <div className={ theme.optionsContainer }>
        { this.search() }
        { this.buttons() }
      </div>
    )
  }

  search() {
    return (
      <div className={ theme.searchContainer }>
        <SearchBox
          value={ this.props.search }
          onChange={ this.props.updateSearch }
        />
      </div>
    )
  }

  buttons() {
    const actions = [
      { label: 'New', accent: true, raised: true, icon: config.icons.add}
    ]
    return (
      <div className={ theme.buttonsContainer }>
        <Navigation type='horizontal' actions={actions} />
      </div>
    )
  }

  page() {
    return (
      <div className={ theme.container }>
        { this.optionsBar() }
        { this.dataList() }
        { this.pager() }
      </div>
    )
  }

  render() {
    const data = this.props.data || []
    return data.length > 0 ? this.page() : this.noData()
  }
}

export default RepoList