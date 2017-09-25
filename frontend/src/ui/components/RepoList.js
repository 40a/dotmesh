import React, { Component, PropTypes } from 'react'

import Navigation from 'react-toolbox/lib/navigation'
import ProgressBar from 'react-toolbox/lib/progress_bar'

import config from '../config'
import RepoListItem from './RepoListItem'
import Pager from './widgets/Pager'
import SearchBox from './widgets/SearchBox'
import FadedText from './widgets/FadedText'
import HelpPage from './widgets/HelpPage'
import theme from './theme/repo.css'

class RepoList extends Component {

  noData() {
    return (
      <div className={ theme.container }>
        <HelpPage
          page='quickstart.md'
          variables={ this.props.helpVariables }
        />
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
                clickRepo={ this.props.clickRepo }
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
        {
          this.props.repoCount > 0 ? (
            <SearchBox
              value={ this.props.search }
              onChange={ this.props.updateSearch }
            />
          ) : (
            <FadedText>
              Once you create some repositories they will display here...
            </FadedText>
          )
        }
      </div>
    )
  }

  buttons() {
    const actions = [
      { label: 'New', accent: true, raised: true, icon: config.icons.add, onClick: () => this.props.clickNew() }
    ]
    return (
      <div className={ theme.buttonsContainer }>
        <Navigation type='horizontal' actions={actions} />
      </div>
    )
  }

  page() {
    return this.props.searchCount > 0 ? (
      <div>
        { this.dataList() }
        { this.pager() }
      </div>
    ) : (
      <div className={ theme.container }>
        No search results...
      </div>
    )
  }

  render() {
    if(!this.props.loaded) {
      return (
        <div>
          <ProgressBar type='circular' mode='indeterminate' multicolor />
        </div>
      )
    }
    return (
      <div id="repoListPage" className={ theme.container }>
        { this.optionsBar() }
        {
          this.props.repoCount > 0 ? this.page() : this.noData()
        }
      </div>
    )
  }
}

export default RepoList